package server

import (
	"database/sql"
	"strconv"

	"bitbucket.org/zlacki/rscgo/pkg/server/errors"
	"bitbucket.org/zlacki/rscgo/pkg/server/world"
	"bitbucket.org/zlacki/rscgo/pkg/strutil"

	// Necessary for sqlite3 driver
	_ "github.com/mattn/go-sqlite3"
)

//ObjectDefinitions This holds the defining characteristics for all of the game's scene objects, ordered by ID.
var ObjectDefinitions []ObjectDefinition

//ObjectDefinition This represents a single definition for a single object in the game.
type ObjectDefinition struct {
	ID            int
	Name          string
	Commands      []string
	Description   string
	Type          int
	Width, Height int
	Length        int
}

//BoundaryDefinitions This holds the defining characteristics for all of the game's boundary scene objects, ordered by ID.
var BoundaryDefinitions []BoundaryDefinition

//BoundaryDefinition This represents a single definition for a single boundary object in the game.
type BoundaryDefinition struct {
	ID          int
	Name        string
	Commands    []string
	Description string
}

//LoadBoundaryDefinitions Loads game boundary object data into memory for quick access.
func LoadBoundaryDefinitions() {
	database := OpenDatabase(TomlConfig.Database.WorldDB)
	defer database.Close()
	// TODO: Seem to be missing a lot of door data.
	rows, err := database.Query("SELECT id, name, description, command_one, command_two FROM `doors`")
	defer rows.Close()
	if err != nil {
		LogError.Println("Couldn't load SQLite3 database:", err)
		return
	}
	for rows.Next() {
		nextDef := BoundaryDefinition{Commands: make([]string, 2)}
		rows.Scan(&nextDef.ID, &nextDef.Name, &nextDef.Description, &nextDef.Commands[0], &nextDef.Commands[1])
		BoundaryDefinitions = append(BoundaryDefinitions, nextDef)
	}
}

//LoadObjectDefinitions Loads game object data into memory for quick access.
func LoadObjectDefinitions() {
	database := OpenDatabase(TomlConfig.Database.WorldDB)
	defer database.Close()
	rows, err := database.Query("SELECT id, name, description, command_one, command_two, type, width, height, ground_item_var FROM `game_objects`")
	defer rows.Close()
	if err != nil {
		LogError.Println("Couldn't load SQLite3 database:", err)
		return
	}
	for rows.Next() {
		nextDef := ObjectDefinition{Commands: make([]string, 2)}
		rows.Scan(&nextDef.ID, &nextDef.Name, &nextDef.Description, &nextDef.Commands[0], &nextDef.Commands[1], &nextDef.Type, &nextDef.Width, &nextDef.Height, &nextDef.Length)
		ObjectDefinitions = append(ObjectDefinitions, nextDef)
	}
}

//LoadObjectLocations Loads the game objects into memory from the SQLite3 database.
func LoadObjectLocations() int {
	objectCounter := 0
	database := OpenDatabase(TomlConfig.Database.WorldDB)
	defer database.Close()
	rows, err := database.Query("SELECT `id`, `direction`, `type`, `x`, `y` FROM `game_object_locations`")
	defer rows.Close()
	if err != nil {
		LogError.Println("Couldn't load SQLite3 database:", err)
		return 0
	}
	var id, direction, boundary, x, y int
	for rows.Next() {
		rows.Scan(&id, &direction, &boundary, &x, &y)
		if world.GetObject(x, y) != nil {
			continue
		}
		objectCounter++
		world.AddObject(world.NewObject(id, direction, x, y, boundary != 0))
	}
	return objectCounter
}

//SaveObjectLocations Clears world.db game object locations and repopulates it with the current server locations.
func SaveObjectLocations() int {
	database := OpenDatabase(TomlConfig.Database.WorldDB)
	defer database.Close()
	tx, err := database.Begin()
	if err != nil {
		LogInfo.Println("Error starting transaction for saving object locations:", err)
		return -1
	}

	stmt, err := tx.Exec("DELETE FROM game_object_locations")
	if err != nil {
		tx.Rollback()
		LogInfo.Println("Error clearing object locations to save new ones:", err)
		return -1
	}
	if count, err := stmt.RowsAffected(); count < 1 || err != nil {
		if err != nil {
			LogWarning.Println("Error inserting new game object location to world.db:", err)
			return -1
		}
		LogWarning.Printf("Rows affected < 1 in game object location insert:%d\n", count)
		return -1
	}

	totalInserts := 0
	for _, v := range world.GetAllObjects() {
		stmt, err := tx.Exec("INSERT INTO game_object_locations(id, direction, x, y, type) VALUES(?, ?, ?, ?, ?)", v.ID, v.Direction, v.X, v.Y, v.Boundary)
		if err != nil {
			LogWarning.Println("Error inserting game object location to database:", err)
			continue
		}
		if count, err := stmt.RowsAffected(); count < 1 || err != nil {
			if err != nil {
				LogWarning.Println("Error inserting new game object location to world.db:", err)
				continue
			}
			LogWarning.Printf("Rows affected < 1 in game object location insert:%d\n", count)
			continue
		}
		totalInserts++
	}

	if err := tx.Commit(); err != nil {
		LogWarning.Println("Couldn't commit game object locations:", err)
		return -1
	}

	return totalInserts
}

//OpenDatabase Returns an active sqlite3 database reference for the specified database file.
func OpenDatabase(file string) *sql.DB {
	database, err := sql.Open("sqlite3", "file:"+TomlConfig.DataDir+file)
	if err != nil {
		LogError.Println("Couldn't load SQLite3 database:", err)
		return nil
	}
	database.SetMaxOpenConns(1)
	return database
}

//UsernameTaken Returns true if there is a player with the name 'username' in the player database, otherwise returns false.
func UsernameTaken(username string) bool {
	database := OpenDatabase(TomlConfig.Database.PlayerDB)
	defer database.Close()
	s, err := database.Query("SELECT id FROM player2 WHERE userhash=?", strutil.Base37(username))
	defer s.Close()
	if err != nil {
		LogInfo.Println("UsernameTaken: Could not query player profile information:", err)
		// return true just to be safe since we could not check
		return true
	}
	if s.Next() {
		// Found a user with this username
		return true
	}
	// Good to go
	return false
}

//CreatePlayer Creates a new entry in the player SQLite3 database with the specified credentials.
// Returns true if successful, otherwise returns false.
func CreatePlayer(username, password string) bool {
	database := OpenDatabase(TomlConfig.Database.PlayerDB)
	defer database.Close()

	tx, err := database.Begin()
	if err != nil {
		LogInfo.Println("CreatePlayer(): Could not begin transaction for new player.")
		return false
	}

	s, err := tx.Exec("INSERT INTO player2(username, userhash, password, x, y, group_id) VALUES(?, ?, ?, 220, 445, 0)", username, strutil.Base37(username), HashPassword(password))

	if err != nil {
		LogInfo.Println("CreatePlayer(): Could not insert new player profile information:", err)
		return false
	}
	playerID, err := s.LastInsertId()
	if err != nil || playerID < 0 {
		LogInfo.Printf("CreatePlayer(): Could not retrieve player database ID(got %d):\n%v", playerID, err)
		return false
	}
	_, err = tx.Exec("INSERT INTO appearance VALUES(?, 2, 8, 14, 0, 1, 2)", playerID)
	if err != nil {
		LogInfo.Println("CreatePlayer(): Could not insert new player profile information:", err)
		return false
	}
	if err := tx.Commit(); err != nil {
		LogWarning.Println("CreatePlayer(): Error committing transaction for new player:", err)
		return false
	}

	return true
}

//LoadPlayer Loads a player from the SQLite3 database, returns a login response code.
func (c *Client) LoadPlayer(usernameHash uint64, password string, loginReply chan byte) {
	validateCredentials := func() error {
		database := OpenDatabase(TomlConfig.Database.PlayerDB)
		defer database.Close()

		stmt, err := database.Prepare("SELECT player.id, player.x, player.y, player.group_id, appearance.haircolour, appearance.topcolour, appearance.trousercolour, appearance.skincolour, appearance.head, appearance.body FROM player2 AS player INNER JOIN appearance AS appearance WHERE appearance.playerid=player.id AND player.userhash=? AND player.password=?")
		defer stmt.Close()
		if err != nil {
			LogInfo.Println("ValidatePlayer(uint64,string): Could not prepare query statement for player:", err)
			loginReply <- byte(3)
			return errors.NewDatabaseError(err.Error())
		}
		rows, err := stmt.Query(usernameHash, password)
		defer rows.Close()
		if err != nil {
			LogInfo.Println("ValidatePlayer(uint64,string): Could not execute query statement for player:", err)
			loginReply <- byte(8)
			return errors.NewDatabaseError(err.Error())
		}
		if !rows.Next() {
			loginReply <- byte(3)
			return errors.NewDatabaseError("Could not find player")
		}
		rows.Scan(&c.player.DatabaseIndex, &c.player.X, &c.player.Y, &c.player.Rank, &c.player.Appearance.Hair, &c.player.Appearance.Top, &c.player.Appearance.Bottom, &c.player.Appearance.Skin, &c.player.Appearance.Head, &c.player.Appearance.Body)
		return nil
	}
	loadAttributes := func() error {
		database := OpenDatabase(TomlConfig.Database.PlayerDB)
		defer database.Close()
		stmt, err := database.Prepare("SELECT name, value FROM player_attr WHERE player_id=?")
		defer stmt.Close()
		if err != nil {
			LogInfo.Println("LoadPlayer(uint64,string): Could not prepare query statement for player attributes:", err)
			return errors.NewDatabaseError("Statement could not be prepared.")
		}
		rows, err := stmt.Query(c.player.DatabaseIndex)
		defer rows.Close()
		if err != nil {
			LogInfo.Println("LoadPlayer(uint64,string): Could not execute query statement for player attributes:", err)
			return errors.NewDatabaseError("Statement could not execute.")
		}
		for rows.Next() {
			var name, value string
			rows.Scan(&name, &value)
			switch value[0] {
			case 'i':
				val, err := strconv.ParseInt(value[1:], 10, 64)
				if err != nil {
					LogInfo.Printf("Error loading int attribute[%v]: value=%v\n", name, value[1:])
					LogInfo.Println(err)
				}
				c.player.Attributes.SetVar(name, int(val))
				break
			case 'l':
				val, err := strconv.ParseUint(value[1:], 10, 64)
				if err != nil {
					LogInfo.Printf("Error loading long int attribute[%v]: value=%v\n", name, value[1:])
					LogInfo.Println(err)
				}
				c.player.Attributes.SetVar(name, uint(val))
				break
			case 'b':
				val, err := strconv.ParseBool(value[1:])
				if err != nil {
					LogInfo.Printf("Error loading boolean attribute[%v]: value=%v\n", name, value[1:])
					LogInfo.Println(err)
				}
				c.player.Attributes.SetVar(name, val)
				break
			}
		}
		return nil
	}
	loadUserList := func(listType string) error {
		database := OpenDatabase(TomlConfig.Database.PlayerDB)
		defer database.Close()
		stmt, err := database.Prepare("SELECT playerhash FROM playerlist WHERE playerid=? AND `type`=?")
		defer stmt.Close()
		if err != nil {
			LogInfo.Println("LoadPlayer(uint64,string): Could not prepare query statement for player friends:", err)
			return errors.NewDatabaseError("Statement could not be prepared.")
		}
		rows, err := stmt.Query(c.player.DatabaseIndex, listType)
		defer rows.Close()
		if err != nil {
			LogInfo.Println("LoadPlayer(uint64,string): Could not execute query statement for player friends:", err)
			return errors.NewDatabaseError("Statement could not execute.")
		}
		for rows.Next() {
			var hash uint64
			rows.Scan(&hash)
			if listType == "friend" {
				c.player.FriendList[hash] = Clients.ContainsHash(hash)
			} else {
				c.player.IgnoreList = append(c.player.IgnoreList, hash)
			}
		}
		return nil
	}
	// If this fails, then the login information was incorrect, and we don't need to do anything else
	if err := validateCredentials(); err != nil {
		return
	}
	if err := loadAttributes(); err != nil {
		return
	}
	if err := loadUserList("friend"); err != nil {
		return
	}
	if err := loadUserList("ignore"); err != nil {
		return
	}

	c.player.UserBase37 = usernameHash
	c.player.Username = strutil.DecodeBase37(usernameHash)
	c.player.Index = c.Index
	Clients.Put(c)
	if c.player.Rank == 2 {
		// Administrator
		loginReply <- byte(25)
		return
	}
	if c.player.Rank == 1 {
		// Moderator
		loginReply <- byte(24)
		return
	}
	loginReply <- byte(0)
	return
}

//Save Saves a player to the SQLite3 database.
func (c *Client) Save() {
	db := OpenDatabase(TomlConfig.Database.PlayerDB)
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		LogInfo.Println("Save(): Could not begin transcaction for player update.")
		return
	}
	saveLocation := func() {
		rs, err := tx.Exec("UPDATE player2 SET x=?, y=? WHERE id=?", c.player.X, c.player.Y, c.player.DatabaseIndex)
		count, err := rs.RowsAffected()
		if err != nil {
			LogWarning.Println("Save(): UPDATE failed for player location:", err)
			if err := tx.Rollback(); err != nil {
				LogWarning.Println("Save(): Transaction location rollback failed:", err)
			}
			return
		}

		if count <= 0 {
			LogInfo.Println("Save(): Affected nothing for location update!")
		}
	}
	saveAppearance := func() {
		// TODO: Should this just be attributes too??  Is that abusing the attributes table?
		appearance := c.player.Appearance
		rs, _ := tx.Exec("UPDATE appearance SET haircolour=?, topcolour=?, trousercolour=?, skincolour=?, head=?, body=? WHERE playerid=?", appearance.Hair, appearance.Top, appearance.Bottom, appearance.Skin, appearance.Head, appearance.Body, c.player.DatabaseIndex)
		count, err := rs.RowsAffected()
		if err != nil {
			LogWarning.Println("Save(): UPDATE failed for player appearance:", err)
			if err := tx.Rollback(); err != nil {
				LogWarning.Println("Save(): Transaction appearance rollback failed:", err)
			}
			return
		}

		if count <= 0 {
			LogInfo.Println("Save(): Affected nothing for appearance update!")
		}
	}
	clearAttributes := func() {
		if _, err := tx.Exec("DELETE FROM player_attr WHERE player_id=?", c.player.DatabaseIndex); err != nil {
			LogWarning.Println("Save(): DELETE failed for player attribute:", err)
			if err := tx.Rollback(); err != nil {
				LogWarning.Println("Save(): Transaction delete attributes rollback failed:", err)
			}
			return
		}
	}
	insertAttribute := func(name string, value interface{}) {
		var val string
		switch value.(type) {
		case int:
			val = "i" + strconv.FormatInt(int64(value.(int)), 10)
		case uint:
			val = "l" + strconv.FormatUint(uint64(value.(uint)), 10)
		case bool:
			if v, ok := value.(bool); v && ok {
				val = "b1"
			} else {
				val = "b0"
			}
		}
		rs, _ := tx.Exec("INSERT INTO player_attr(player_id, name, value) VALUES(?, ?, ?)", c.player.DatabaseIndex, name, val)
		count, err := rs.RowsAffected()
		if err != nil {
			LogWarning.Println("Save(): INSERT failed for player attribute:", err)
			if err := tx.Rollback(); err != nil {
				LogWarning.Println("Save(): Transaction insert attribute rollback failed:", err)
			}
			return
		}

		if count <= 0 {
			LogInfo.Println("Save(): Affected nothing for attribute insertion!")
		}
	}
	clearContactList := func(contactType string) {
		if _, err := tx.Exec("DELETE FROM playerlist WHERE playerid=? AND type=?", c.player.DatabaseIndex, contactType); err != nil {
			LogWarning.Println("Save(): DELETE failed for player friends:", err)
			if err := tx.Rollback(); err != nil {
				LogWarning.Println("Save(): Transaction delete friends rollback failed:", err)
			}
			return
		}
	}
	insertContactList := func(contactType string, hash uint64) {
		rs, _ := tx.Exec("INSERT INTO playerlist(playerid, playerhash, type) VALUES(?, ?, ?)", c.player.DatabaseIndex, hash, contactType)
		count, err := rs.RowsAffected()
		if err != nil {
			LogWarning.Println("Save(): INSERT failed for player friends:", err)
			if err := tx.Rollback(); err != nil {
				LogWarning.Println("Save(): Transaction insert friend rollback failed:", err)
			}
			return
		}

		if count <= 0 {
			LogInfo.Println("Save(): Affected nothing for friend insertion!")
		}
	}
	saveLocation()
	saveAppearance()
	clearAttributes()
	c.player.Attributes.Range(insertAttribute)
	clearContactList("friend")
	clearContactList("ignore")
	for hash := range c.player.FriendList {
		insertContactList("friend", hash)
	}
	for _, hash := range c.player.IgnoreList {
		insertContactList("ignore", hash)
	}

	if err := tx.Commit(); err != nil {
		LogWarning.Println("Save(): Error committing transaction for player update:", err)
	}
}
