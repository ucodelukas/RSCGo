package world

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/spkaeros/rscgo/pkg/game/net"
	"github.com/spkaeros/rscgo/pkg/strutil"
)

//AppearanceTable Represents a players appearance.
type AppearanceTable struct {
	Head      int
	Body      int
	Legs      int
	Male      bool
	HeadColor int
	BodyColor int
	LegsColor int
	SkinColor int
}

//NewAppearanceTable returns a reference to a new appearance table with specified parameters
func NewAppearanceTable(head, body int, male bool, hair, top, bottom, skin int) AppearanceTable {
	return AppearanceTable{head, body, 3, male, hair, top, bottom, skin}
}

func DefaultAppearance() AppearanceTable {
	return NewAppearanceTable(1, 2, true, 2, 8, 14, 0)
}

//player Represents a single player.
type Player struct {
	LocalPlayers     *entityList
	LocalNPCs        *entityList
	LocalObjects     *entityList
	LocalItems       *entityList
	FriendList       map[uint64]bool
	IgnoreList       []uint64
	Appearance       AppearanceTable
	KnownAppearances map[int]int
	AppearanceReq    []*Player
	AppearanceLock   sync.RWMutex
	Attributes       *AttributeList
	Inventory        *Inventory
	TradeOffer       *Inventory
	DuelOffer        *Inventory
	DistancedAction  func() bool
	ActionLock       sync.RWMutex
	OutgoingPackets  chan *net.Packet
	ReplyMenuC       chan int8
	Equips           [12]int
	killer           sync.Once
	KillC            chan struct{}
	Tickables        []interface{}
	*Mob
}

func (p *Player) UsernameHash() uint64 {
	return p.TransAttrs.VarLong("username", strutil.Base37.Encode("nil"))
}

func (p *Player) Bank() *Inventory {
	i, ok := p.TransAttrs.Var("bank")
	if ok {
		return i.(*Inventory)
	}
	return nil
}

func (p *Player) Username() string {
	return strutil.Base37.Decode(p.TransAttrs.VarLong("username", strutil.Base37.Encode("NIL")))
}

func (p *Player) CurrentIP() string {
	return p.TransAttrs.VarString("currentIP", "0.0.0.0")
}

func (p *Player) Rank() int {
	return p.TransAttrs.VarInt("rank", 0)
}

func (p *Player) DatabaseID() int {
	return p.TransAttrs.VarInt("dbID", -1)
}

func (p *Player) AppearanceTicket() int {
	return p.TransAttrs.VarInt("appearanceTicket", 0)
}

//String returns a string populated with the more identifying features of this player.
func (p *Player) String() string {
	return fmt.Sprintf("Player[%d] {'%v'@'%v'}", p.Index, p.Username(), p.CurrentIP())
}

//SetDistancedAction queues a distanced action to run every game engine tick before path traversal, if action returns true, it will be reset.
func (p *Player) SetDistancedAction(action func() bool) {
	p.ActionLock.Lock()
	p.DistancedAction = action
	p.ActionLock.Unlock()
}

//ResetDistancedAction clears the distanced action, if any is queued.  Should be called any time the player is deliberately performing an action.
func (p *Player) ResetDistancedAction() {
	p.ActionLock.Lock()
	p.DistancedAction = nil
	p.ActionLock.Unlock()
}

//Friends returns true if specified username is in our friend entityList.
func (p *Player) Friends(other uint64) bool {
	for hash := range p.FriendList {
		if hash == other {
			return true
		}
	}
	return false
}

//Ignoring returns true if specified username is in our ignore entityList.
func (p *Player) Ignoring(hash uint64) bool {
	for _, v := range p.IgnoreList {
		if v == hash {
			return true
		}
	}
	return false
}

//ChatBlocked returns true if public chat is blocked for this player.
func (p *Player) ChatBlocked() bool {
	return p.Attributes.VarBool("chat_block", false)
}

//FriendBlocked returns true if private chat is blocked for this player.
func (p *Player) FriendBlocked() bool {
	return p.Attributes.VarBool("friend_block", false)
}

//TradeBlocked returns true if trade requests are blocked for this player.
func (p *Player) TradeBlocked() bool {
	return p.Attributes.VarBool("trade_block", false)
}

//DuelBlocked returns true if duel requests are blocked for this player.
func (p *Player) DuelBlocked() bool {
	return p.Attributes.VarBool("duel_block", false)
}

//SetPrivacySettings sets privacy settings to specified values.
func (p *Player) SetPrivacySettings(chatBlocked, friendBlocked, tradeBlocked, duelBlocked bool) {
	p.Attributes.SetVar("chat_block", chatBlocked)
	p.Attributes.SetVar("friend_block", friendBlocked)
	p.Attributes.SetVar("trade_block", tradeBlocked)
	p.Attributes.SetVar("duel_block", duelBlocked)
}

//SetClientSetting sets the specified client setting to flag.
func (p *Player) SetClientSetting(id int, flag bool) {
	// TODO: Meaningful names mapped to IDs
	p.Attributes.SetVar("client_setting_"+strconv.Itoa(id), flag)
}

//GetClientSetting looks up the client setting with the specified ID, and returns it.  If it can't be found, returns false.
func (p *Player) GetClientSetting(id int) bool {
	// TODO: Meaningful names mapped to IDs
	return p.Attributes.VarBool("client_setting_"+strconv.Itoa(id), false)
}

//IsFollowing returns true if the player is following another mob, otherwise false.
func (p *Player) IsFollowing() bool {
	return p.FollowRadius() >= 0
}

//ServerSeed returns the seed for the ISAAC cipher provided by the game for this player, if set, otherwise returns 0
func (p *Player) ServerSeed() uint64 {
	return p.TransAttrs.VarLong("server_seed", 0)
}

//SetServerSeed sets the player's stored game seed to seed for later comparison to ensure we decrypted the login block properly and the player received the proper seed.
func (p *Player) SetServerSeed(seed uint64) {
	p.TransAttrs.SetVar("server_seed", seed)
}

//Reconnecting returns true if the player is reconnecting, false otherwise.
func (p *Player) Reconnecting() bool {
	return p.TransAttrs.VarBool("reconnecting", false)
}

//SetReconnecting sets the player's reconnection status to flag.
func (p *Player) SetReconnecting(flag bool) {
	p.TransAttrs.SetVar("reconnecting", flag)
}

//Connected returns true if the player is connected, false otherwise.
func (p *Player) Connected() bool {
	return p.TransAttrs.VarBool("connected", false)
}

//SetConnected sets the player's connected status to flag.
func (p *Player) SetConnected(flag bool) {
	p.TransAttrs.SetVar("connected", flag)
}

//FirstLogin returns true if this player has never logged in before, otherwise false.
func (p *Player) FirstLogin() bool {
	return p.Attributes.VarBool("first_login", true)
}

//SetFirstLogin sets the player's persistent logged in before status to flag.
func (p *Player) SetFirstLogin(flag bool) {
	p.Attributes.SetVar("first_login", flag)
}

//StartFollowing sets the transient attribute for storing the radius with which we want to stay near our target
func (p *Player) StartFollowing(radius int) {
	p.TransAttrs.SetVar("followrad", radius)
}

//FollowRadius returns the radius within which we should follow whatever mob we are following, or -1 if we aren't following anyone.
func (p *Player) FollowRadius() int {
	return p.TransAttrs.VarInt("followrad", -1)
}

//ResetFollowing resets the transient attribute for storing the radius within which we want to stay to our target mob
// and resets our path.
func (p *Player) ResetFollowing() {
	p.TransAttrs.UnsetVar("followrad")
	p.ResetPath()
}

//NextTo returns true if we can walk a straight line to target without colliding with any walls or objects,
// otherwise returns false.
func (p *Player) NextTo(target Location) bool {
	if p.X() > target.X() {
		if IsTileBlocking(p.X(), p.Y(), ClipEast, true) {
			return false
		}
		if IsTileBlocking(target.X(), target.Y(), ClipWest, false) {
			return false
		}
	} else if p.X() < target.X() {
		if IsTileBlocking(p.X(), p.Y(), ClipWest, true) {
			return false
		}
		if IsTileBlocking(target.X(), target.Y(), ClipEast, false) {
			return false
		}
	}
	if p.Y() > target.Y() {
		if IsTileBlocking(p.X(), p.Y(), ClipNorth, true) {
			return false
		}
		if IsTileBlocking(target.X(), target.Y(), ClipSouth, false) {
			return false
		}
	} else if p.Y() < target.Y() {
		if IsTileBlocking(p.X(), p.Y(), ClipSouth, true) {
			return false
		}
		if IsTileBlocking(target.X(), target.Y(), ClipNorth, false) {
			return false
		}
	}

	return true
}

func (p *Player) NextToCoords(x, y int) bool {
	return p.NextTo(NewLocation(x, y))
}

//TraversePath if the mob has a path, calling this method will change the mobs location to the next location described by said Path data structure.  This should be called no more than once per game tick.
func (p *Player) TraversePath() {
	path := p.Path()
	if path == nil {
		return
	}
	if p.AtLocation(path.nextTile()) {
		path.CurrentWaypoint++
	}
	if p.FinishedPath() {
		p.ResetPath()
		return
	}
	dst := path.nextTile()
	x, y := p.X(), p.Y()
	next := NewLocation(x, y)
	xBlocked, yBlocked := false, false
	newXBlocked, newYBlocked := false, false
	if y > dst.Y() {
		yBlocked = IsTileBlocking(x, y, ClipNorth, true)
		newYBlocked = IsTileBlocking(x, y-1, ClipSouth, false)
		if !newYBlocked {
			next.y.Dec()
		}
	} else if y < dst.Y() {
		yBlocked = IsTileBlocking(x, y, ClipSouth, true)
		newYBlocked = IsTileBlocking(x, y+1, ClipNorth, false)
		if !newYBlocked {
			next.y.Inc()
		}
	}
	if x > dst.X() {
		xBlocked = IsTileBlocking(x, next.Y(), ClipEast, true)
		newXBlocked = IsTileBlocking(x-1, next.Y(), ClipWest, false)
		if !newXBlocked {
			next.x.Dec()
		}
	} else if x < dst.X() {
		xBlocked = IsTileBlocking(x, next.Y(), ClipWest, true)
		newXBlocked = IsTileBlocking(x+1, next.Y(), ClipEast, false)
		if !newXBlocked {
			next.x.Inc()
		}
	}

	if (xBlocked && yBlocked) || (xBlocked && y == dst.Y()) || (yBlocked && x == dst.X()) {
		p.ResetPath()
		return
	}
	if (newXBlocked && newYBlocked) || (newXBlocked && x != next.X() && y == next.Y()) || (newYBlocked && y != next.Y() && x == next.X()) {
		p.ResetPath()
		return
	}

	if next.X() > x {
		newXBlocked = IsTileBlocking(next.X(), next.Y(), ClipEast, false)
	} else if next.X() < x {
		newXBlocked = IsTileBlocking(next.X(), next.Y(), ClipWest, false)
	}
	if next.Y() > y {
		newYBlocked = IsTileBlocking(next.X(), next.Y(), ClipNorth, false)
	} else if next.Y() < y {
		newYBlocked = IsTileBlocking(next.X(), next.Y(), ClipSouth, false)
	}

	if (newXBlocked && newYBlocked) || (newXBlocked && y == next.Y()) || (newYBlocked && x == next.X()) {
		p.ResetPath()
		return
	}

	p.SetLocation(next, false)
}

//UpdateRegion if this player is currently in a region, removes it from that region, and adds it to the region at x,y
func (p *Player) UpdateRegion(x, y int) {
	curArea := getRegion(p.X(), p.Y())
	newArea := getRegion(x, y)
	if newArea != curArea {
		if curArea.Players.Contains(p) {
			curArea.Players.Remove(p)
		}
		newArea.Players.Add(p)
	}
}

//DistributeMeleeExp This is a helper method to distribute experience amongst the players melee stats according to
// its current fight stance.
//
// If the player is in controlled stance, each melee skill gets (experience).
// Otherwise, whatever fight stance the player was in will get (experience)*3, and hits will get (experience).
func (p *Player) DistributeMeleeExp(experience int) {
	switch p.FightMode() {
	case 0:
		for i := 0; i < 3; i++ {
			p.IncExp(i, experience)
		}
	case 1:
		p.IncExp(StatStrength, experience*3)
	case 2:
		p.IncExp(StatAttack, experience*3)
	case 3:
		p.IncExp(StatDefense, experience*3)
	}
	p.IncExp(StatHits, experience)
}

//EquipItem equips an item to this player, and sends inventory and equipment bonuses.
func (p *Player) EquipItem(item *Item) {
	def := GetEquipmentDefinition(item.ID)
	if def == nil {
		return
	}
	if def.Female && p.Appearance.Male {
		// TODO: Look up canonical message
		p.Message("You must be a female to wear that")
		return
	}
	p.Inventory.Range(func(otherItem *Item) bool {
		otherDef := GetEquipmentDefinition(otherItem.ID)
		if otherItem == item || !otherItem.Worn || otherDef == nil || def.Type&otherDef.Type == 0 {
			return true
		}
		p.SetAimPoints(p.AimPoints() - otherDef.Aim)
		p.SetPowerPoints(p.PowerPoints() - otherDef.Power)
		p.SetArmourPoints(p.ArmourPoints() - otherDef.Armour)
		p.SetMagicPoints(p.MagicPoints() - otherDef.Magic)
		p.SetPrayerPoints(p.PrayerPoints() - otherDef.Prayer)
		p.SetRangedPoints(p.RangedPoints() - otherDef.Ranged)
		otherItem.Worn = false
		p.AppearanceLock.Lock()
		if otherDef.Type&1 == 1 {
			p.Equips[otherDef.Position] = p.Appearance.Head
		} else if otherDef.Type&2 == 2 {
			p.Equips[otherDef.Position] = p.Appearance.Body
		} else if otherDef.Type&4 == 4 {
			p.Equips[otherDef.Position] = p.Appearance.Legs
		} else {
			p.Equips[otherDef.Position] = 0
		}
		p.AppearanceLock.Unlock()
		return true
	})
	item.Worn = true
	p.SetAimPoints(p.AimPoints() + def.Aim)
	p.SetPowerPoints(p.PowerPoints() + def.Power)
	p.SetArmourPoints(p.ArmourPoints() + def.Armour)
	p.SetMagicPoints(p.MagicPoints() + def.Magic)
	p.SetPrayerPoints(p.PrayerPoints() + def.Prayer)
	p.SetRangedPoints(p.RangedPoints() + def.Ranged)
	p.AppearanceLock.Lock()
	p.Equips[def.Position] = def.Sprite
	p.AppearanceLock.Unlock()
	p.UpdateAppearance()
}

func (p *Player) UpdateAppearance() {
	p.SetAppearanceChanged()
	p.TransAttrs.SetVar("appearanceTicket", p.AppearanceTicket()+1)
}

//DequipItem removes an item from this players equips, and sends inventory and equipment bonuses.
func (p *Player) DequipItem(item *Item) {
	def := GetEquipmentDefinition(item.ID)
	if def == nil {
		return
	}
	if !item.Worn {
		return
	}
	item.Worn = false
	p.SetAimPoints(p.AimPoints() - def.Aim)
	p.SetPowerPoints(p.PowerPoints() - def.Power)
	p.SetArmourPoints(p.ArmourPoints() - def.Armour)
	p.SetMagicPoints(p.MagicPoints() - def.Magic)
	p.SetPrayerPoints(p.PrayerPoints() - def.Prayer)
	p.SetRangedPoints(p.RangedPoints() - def.Ranged)
	p.AppearanceLock.Lock()
	if def.Type&1 == 1 {
		p.Equips[def.Position] = p.Appearance.Head
	} else if def.Type&2 == 2 {
		p.Equips[def.Position] = p.Appearance.Body
	} else if def.Type&4 == 4 {
		p.Equips[def.Position] = p.Appearance.Legs
	} else {
		p.Equips[def.Position] = 0
	}
	p.AppearanceLock.Unlock()
	p.UpdateAppearance()
}

//ResetAll in order, calls ResetFighting, ResetTrade, ResetDistancedAction, ResetFollowing, and CloseOptionMenu.
func (p *Player) ResetAll() {
	p.ResetFighting()
	p.ResetTrade()
	p.ResetDistancedAction()
	p.ResetFollowing()
	p.CloseOptionMenu()
	p.ResetDuel()
	p.CloseBank()
	p.CloseShop()
}

//Fatigue Returns the players current fatigue.
func (p *Player) Fatigue() int {
	return p.Attributes.VarInt("fatigue", 0)
}

//SetFatigue Sets the players current fatigue to i.
func (p *Player) SetFatigue(i int) {
	p.Attributes.SetVar("fatigue", i)
}

//NearbyPlayers Returns nearby players.
func (p *Player) NearbyPlayers() (players []*Player) {
	for _, r := range surroundingRegions(p.X(), p.Y()) {
		players = append(players, r.Players.NearbyPlayers(p)...)
	}

	return
}

//NearbyNpcs Returns nearby NPCs.
func (p *Player) NearbyNpcs() (npcs []*NPC) {
	for _, r := range surroundingRegions(p.X(), p.Y()) {
		npcs = append(npcs, r.NPCs.NearbyNpcs(p)...)
	}

	return
}

//NearbyObjects Returns nearby objects.
func (p *Player) NearbyObjects() (objects []*Object) {
	for _, r := range surroundingRegions(p.X(), p.Y()) {
		objects = append(objects, r.Objects.NearbyObjects(p)...)
	}

	return
}

//NewObjects Returns nearby objects that this player is unaware of.
func (p *Player) NewObjects() (objects []*Object) {
	for _, r := range surroundingRegions(p.X(), p.Y()) {
		for _, o := range r.Objects.NearbyObjects(p) {
			if !p.LocalObjects.Contains(o) {
				objects = append(objects, o)
			}
		}
	}

	return
}

//NewItems Returns nearby ground items that this player is unaware of.
func (p *Player) NewItems() (items []*GroundItem) {
	for _, r := range surroundingRegions(p.X(), p.Y()) {
		for _, i := range r.Items.NearbyItems(p) {
			if !p.LocalItems.Contains(i) {
				items = append(items, i)
			}
		}
	}

	return
}

//NewPlayers Returns nearby players that this player is unaware of.
func (p *Player) NewPlayers() (players []*Player) {
	for _, r := range surroundingRegions(p.X(), p.Y()) {
		for _, p1 := range r.Players.NearbyPlayers(p) {
			if !p.LocalPlayers.Contains(p1) {
				players = append(players, p1)
			}
		}
	}

	return
}

//NewNPCs Returns nearby NPCs that this player is unaware of.
func (p *Player) NewNPCs() (npcs []*NPC) {
	for _, r := range surroundingRegions(p.X(), p.Y()) {
		for _, n := range r.NPCs.NearbyNpcs(p) {
			if !p.LocalNPCs.Contains(n) {
				npcs = append(npcs, n)
			}
		}
	}

	return
}

//SetTradeTarget Sets the variable for the index of the player we are trying to trade
func (p *Player) SetTradeTarget(index int) {
	p.TransAttrs.SetVar("tradetarget", index)
}

//IsTrading returns true if this player is in a trade, otherwise returns false.
func (p *Player) IsTrading() bool {
	return p.HasState(MSTrading)
}

//ResetTrade resets trade-related variables.
func (p *Player) ResetTrade() {
	if p.IsTrading() {
		p.TransAttrs.UnsetVar("tradetarget")
		p.TransAttrs.UnsetVar("trade1accept")
		p.TransAttrs.UnsetVar("trade2accept")
		p.TradeOffer.Clear()
		p.RemoveState(MSTrading)
	}
}

//TradeTarget returns the game index of the player we are trying to trade with, or -1 if we have not made a trade request.
func (p *Player) TradeTarget() int {
	return p.TransAttrs.VarInt("tradetarget", -1)
}

//CombatDelta returns the difference between our combat level and the other mobs combat level
func (p *Player) CombatDelta(other MobileEntity) int {
	return p.Skills().CombatLevel() - other.Skills().CombatLevel()
}

//ResetDuel resets duel-related variables.
func (p *Player) ResetDuel() {
	if p.IsDueling() {
		p.ResetDuelTarget()
		p.ResetDuelAccepted()
		p.DuelOffer.Clear()
		p.TransAttrs.UnsetVar("duelCanRetreat")
		p.TransAttrs.UnsetVar("duelCanMagic")
		p.TransAttrs.UnsetVar("duelCanPrayer")
		p.TransAttrs.UnsetVar("duelCanEquip")
		p.RemoveState(MSDueling)
	}
}

//IsDueling returns true if this player is negotiating a duel, otherwise returns false.
func (p *Player) IsDueling() bool {
	return p.HasState(MSDueling)
}

//SetDuelTarget Sets p1 as the receivers dueling target.
func (p *Player) SetDuelTarget(p1 *Player) {
	p.TransAttrs.SetVar("duelTarget", p1)
}

//ResetDuelTarget Removes receivers duel target, if any.
func (p *Player) ResetDuelTarget() {
	p.TransAttrs.UnsetVar("duelTarget")
}

//ResetDuelAccepted Resets receivers duel negotiation settings to indicate that neither screens are accepted.
func (p *Player) ResetDuelAccepted() {
	p.TransAttrs.UnsetVar("duel1accept")
	p.TransAttrs.UnsetVar("duel2accept")
}

//SetDuel1Accepted Sets receivers duel negotiation settings to indicate that the first screen is accepted.
func (p *Player) SetDuel1Accepted() {
	p.TransAttrs.SetVar("duel1accept", true)
}

//SetDuel2Accepted Sets receivers duel negotiation settings to indicate that the second screen is accepted.
func (p *Player) SetDuel2Accepted() {
	p.TransAttrs.SetVar("duel2accept", true)
}

//DuelTarget Returns the player that the receiver is targeting to duel with, or if none, returns nil
func (p *Player) DuelTarget() *Player {
	return p.TransAttrs.VarPlayer("duelTarget")
}

//SendPacket sends a net to the client.
func (p *Player) SendPacket(packet *net.Packet) {
	if p == nil || !p.Connected() {
		return
	}
	p.OutgoingPackets <- packet
}

//Destroy sends a kill signal to the underlying client to tear down all of the I/O routines and save the player.
func (p *Player) Destroy() {
	p.killer.Do(func() {
		p.ResetAll()
		p.Attributes.SetVar("lastIP", p.CurrentIP())
		p.Inventory.Owner = nil
		close(p.KillC)
	})
}

func (p *Player) AtObject(object *Object) bool {
	x, y := p.X(), p.Y()
	bounds := object.Boundaries()
	if ObjectDefs[object.ID].Type == 2 || ObjectDefs[object.ID].Type == 3 {
		return (p.NextTo(bounds[0]) || p.NextTo(bounds[1])) && (x >= bounds[0].X() && x <= bounds[1].X() && y >= bounds[0].Y() && y <= bounds[1].Y())
	}

	return p.CanReach(bounds) || (p.FinishedPath() && p.CanReachDiag(bounds))
}

func (p *Player) CanReach(bounds [2]Location) bool {
	x, y := p.X(), p.Y()

	if x >= bounds[0].X() && x <= bounds[1].X() && y >= bounds[0].Y() && y <= bounds[1].Y() {
		return true
	}
	if x-1 >= bounds[0].X() && x-1 <= bounds[1].X() && y >= bounds[0].Y() && y <= bounds[1].Y() &&
		(CollisionData(x-1, y).CollisionMask&ClipWest) == 0 {
		return true
	}
	if x+1 >= bounds[0].X() && x+1 <= bounds[1].X() && y >= bounds[0].Y() && y <= bounds[1].Y() &&
		(CollisionData(x+1, y).CollisionMask&ClipEast) == 0 {
		return true
	}
	if x >= bounds[0].X() && x <= bounds[1].X() && bounds[0].Y() <= y-1 && bounds[1].Y() >= y-1 &&
		(CollisionData(x, y-1).CollisionMask&ClipSouth) == 0 {
		return true
	}
	if x >= bounds[0].X() && x <= bounds[1].X() && bounds[0].Y() <= y+1 && bounds[1].Y() >= y+1 &&
		(CollisionData(x, y-1).CollisionMask&ClipNorth) == 0 {
		return true
	}
	return false
}

func (p *Player) CanReachDiag(bounds [2]Location) bool {
	x, y := p.X(), p.Y()
	if x-1 >= bounds[0].X() && x-1 <= bounds[1].X() && y-1 >= bounds[0].Y() && y-1 <= bounds[1].Y() &&
		(CollisionData(x-1, y-1).CollisionMask&ClipSouth|ClipWest) == 0 {
		return true
	}
	if x-1 >= bounds[0].X() && x-1 <= bounds[1].X() && y+1 >= bounds[0].Y() && y+1 <= bounds[1].Y() &&
		(CollisionData(x-1, y+1).CollisionMask&ClipNorth|ClipWest) == 0 {
		return true
	}
	if x+1 >= bounds[0].X() && x+1 <= bounds[1].X() && y-1 >= bounds[0].Y() && y-1 <= bounds[1].Y() &&
		(CollisionData(x+1, y-1).CollisionMask&ClipSouth|ClipEast) == 0 {
		return true
	}
	if x+1 >= bounds[0].X() && x+1 <= bounds[1].X() && y+1 >= bounds[0].Y() && y+1 <= bounds[1].Y() &&
		(CollisionData(x+1, y+1).CollisionMask&ClipNorth|ClipEast) == 0 {
		return true
	}

	return false
}

func (p *Player) SendFatigue() {
	p.SendPacket(Fatigue(p))
}

//Initialize informs the client of all of the various attributes of this player, and starts the stat normalization
// routine.
func (p *Player) Initialize() {
	p.SetAppearanceChanged()
	p.SetSpriteUpdated()
	AddPlayer(p)
	p.SendPacket(FriendList(p))
	p.SendPacket(IgnoreList(p))
	p.SendPlane()
	p.SendEquipBonuses()
	p.SendInventory()
	p.SendFatigue()
	// TODO: Not canonical RSC, but definitely good QoL update...
	//  p.SendPacket(FightMode(p))
	p.SendPacket(ClientSettings(p))
	p.SendPacket(PrivacySettings(p))
	if p.FirstLogin() {
		p.SetFirstLogin(false)
		for i := 0; i < 18; i++ {
			if i != 3 {
				p.Skills().SetCur(i, 1)
				p.Skills().SetMax(i, 1)
				p.Skills().SetExp(i, 0)
			}
		}
		p.Skills().SetCur(StatHits, 10)
		p.Skills().SetMax(StatHits, 10)
		p.Skills().SetExp(StatHits, LevelToExperience(10))
		p.OpenAppearanceChanger()
	}
	if !p.Reconnecting() {
		p.SendPacket(WelcomeMessage)
		if timestamp := p.Attributes.VarTime("lastLogin"); !timestamp.IsZero() {
			p.SendPacket(LoginBox(int(time.Since(timestamp).Hours()/24), p.Attributes.VarString("lastIP", "0.0.0.0")))
		}
	}
	p.SendStats()
	p.Attributes.SetVar("lastLogin", time.Now())
}

//NewPlayer Returns a reference to a new player.
func NewPlayer(index int, ip string) *Player {
	p := &Player{Mob: &Mob{Entity: &Entity{Index: index, Location: Lumbridge.Clone()}, TransAttrs: NewAttributeList()},
		Attributes: NewAttributeList(), LocalPlayers: &entityList{}, LocalNPCs: &entityList{}, LocalObjects: &entityList{},
		Appearance: DefaultAppearance(), FriendList: make(map[uint64]bool), KnownAppearances: make(map[int]int),
		Inventory: &Inventory{Capacity: 30}, TradeOffer: &Inventory{Capacity: 12}, DuelOffer: &Inventory{Capacity: 8},
		LocalItems: &entityList{}, OutgoingPackets: make(chan *net.Packet, 20), KillC: make(chan struct{})}
	p.Transients().SetVar("skills", &SkillTable{})
	p.Transients().SetVar("bank", &Inventory{Capacity: 48 * 4, stackEverything: true})
	p.Transients().SetVar("viewRadius", 16)
	p.Transients().SetVar("currentIP", ip)

	p.Equips[0] = p.Appearance.Head
	p.Equips[1] = p.Appearance.Body
	p.Equips[2] = p.Appearance.Legs
	p.Inventory.Owner = p
	return p
}

//Message sends a message to the player.
func (p *Player) Message(msg string) {
	p.SendPacket(ServerMessage(msg))
}

//OpenAppearanceChanger If the player is not fighting or trading, opens the appearance window.
func (p *Player) OpenAppearanceChanger() {
	if p.IsFighting() || p.IsTrading() {
		return
	}
	p.AddState(MSChangingAppearance)
	p.SendPacket(OpenChangeAppearance)
}

//Chat sends a player NPC chat message net to the player and all other players around it.  If multiple msgs are
// provided, will sleep the goroutine for 1800ms between each message.
func (p *Player) Chat(msgs ...string) {
	for _, msg := range msgs {
		for _, player := range p.NearbyPlayers() {
			player.SendPacket(PlayerMessage(p, msg))
		}
		p.SendPacket(PlayerMessage(p, msg))

		//		if i < len(msgs)-1 {
		time.Sleep(time.Millisecond * 1920)
		// TODO: is 3 ticks right?
		//		}
	}
}

//OpenOptionMenu opens an option menu with the provided options, and returns the reply index, or -1 upon timeout..
func (p *Player) OpenOptionMenu(options ...string) int {
	// Can get option menu during most states, even fighting, but not trading, or if we're already in a menu...
	if p.IsTrading() || p.HasState(MSOptionMenu) {
		return -1
	}
	p.ReplyMenuC = make(chan int8)
	p.AddState(MSOptionMenu)
	p.SendPacket(OptionMenuOpen(options...))

	select {
	case reply := <-p.ReplyMenuC:
		if !p.HasState(MSOptionMenu) {
			return -1
		}
		p.RemoveState(MSOptionMenu)
		close(p.ReplyMenuC)
		if reply < 0 || int(reply) > len(options)-1 {
			return -1
		}

		if p.HasState(MSChatting) {
			p.Chat(options[reply])
		}
		return int(reply)
	case <-time.After(time.Second * 20):
		if p.HasState(MSOptionMenu) {
			p.RemoveState(MSOptionMenu)
			close(p.ReplyMenuC)
			p.SendPacket(OptionMenuClose)
		}
		return -1
	}
}

//CloseOptionMenu closes any open option menus.
func (p *Player) CloseOptionMenu() {
	if p.HasState(MSOptionMenu) {
		p.RemoveState(MSOptionMenu)
		close(p.ReplyMenuC)
		p.SendPacket(OptionMenuClose)
	}
}

//CanWalk returns true if this player is in a state that allows walking.
func (p *Player) CanWalk() bool {
	if p.HasState(MSOptionMenu) && (p.HasState(MSChatting) || p.HasState(MSItemAction)) {
		// If player tries to walk but is in an option menu, they clearly have closed the menu, so we will kill the
		// routine waiting for a reply when ResetAll is called before the new path is set.
		return true
	}
	return !p.HasState(MSBatching, MSFighting, MSTrading, MSDueling, MSChangingAppearance, MSSleeping, MSChatting, MSBusy, MSItemAction, MSShopping)
}

//PlaySound sends a command to the client to play a sound by its file name.
func (p *Player) PlaySound(soundName string) {
	p.SendPacket(Sound(soundName))
}

//SendStat sends the information for the stat at idx to the player.
func (p *Player) SendStat(idx int) {
	p.SendPacket(PlayerStat(p, idx))
}

//SendStatExp sends the experience information for the stat at idx to the player.
func (p *Player) SendStatExp(idx int) {
	p.SendPacket(PlayerExperience(p, idx))
}

//SendStats sends all stat information to this player.
func (p *Player) SendStats() {
	p.SendPacket(PlayerStats(p))
}

//SendInventory sends inventory information to this player.
func (p *Player) SendInventory() {
	p.SendPacket(InventoryItems(p))
}

//SetCurStat sets this players current stat at idx to lvl and updates the client about it.
func (p *Player) SetCurStat(idx int, lvl int) {
	p.Skills().SetCur(idx, lvl)
	p.SendStat(idx)
}

//IncCurStat sets this players current stat at idx to Current(idx)+lvl and updates the client about it.
func (p *Player) IncCurStat(idx int, lvl int) {
	p.Skills().IncreaseCur(idx, lvl)
	p.SendStat(idx)
}

//SetCurStat sets this players current stat at idx to lvl and updates the client about it.
func (p *Player) IncExp(idx int, amt int) {
	// TODO: Fatigue
	p.Skills().IncExp(idx, amt)
	delta := ExperienceToLevel(p.Skills().Experience(idx)) - p.Skills().Maximum(idx)
	if delta != 0 {
		p.Message(fmt.Sprintf("@gre@You just advanced %d %v level!", delta, SkillName(idx)))
		p.PlaySound("advance")
		oldCombat := p.Skills().CombatLevel()
		p.Skills().IncreaseCur(idx, delta)
		p.Skills().IncreaseMax(idx, delta)
		p.SendStat(idx)
		if oldCombat != p.Skills().CombatLevel() {
			p.UpdateAppearance()
		}
	} else {
		p.SendStatExp(idx)
	}
}

//SetMaxStat sets this players maximum stat at idx to lvl and updates the client about it.
func (p *Player) SetMaxStat(idx int, lvl int) {
	p.Skills().SetMax(idx, lvl)
	p.Skills().SetExp(idx, LevelToExperience(lvl))
	p.SendStat(idx)
}

//AddItem Adds amount of the item with specified id to the players inventory, if possible, and updates the client about it.
func (p *Player) AddItem(id, amount int) {
	if !ItemDefs[id].Stackable {
		for i := 0; i < amount; i++ {
			if p.Inventory.Size() >= p.Inventory.Capacity {
				item := NewGroundItemFor(p.UsernameHash(), id, 1, p.X(), p.Y())
				AddItem(item)
				p.Message("Your inventory is full, the " + item.Name() + " drops to the ground!")
			} else {
				p.Inventory.Add(id, 1)
			}
		}
	} else {
		if p.Inventory.Size() >= p.Inventory.Capacity {
			item := NewGroundItemFor(p.UsernameHash(), id, amount, p.X(), p.Y())
			AddItem(item)
			p.Message("Your inventory is full, the " + item.Name() + " drops to the ground!")
		} else {
			p.Inventory.Add(id, amount)
		}
	}
	p.SendInventory()
}

func (p *Player) PrayerActivated(idx int) bool {
	return p.TransAttrs.VarBool("prayer"+strconv.Itoa(idx), false)
}

func (p *Player) PrayerOn(idx int) {
	if p.IsDueling() && !p.TransAttrs.VarBool("duelCanPrayer", true) {
		p.Message("You cannot use prayer in this duel!")
		p.SendPrayers()
		return
	}
	if idx == 0 || idx == 3 || idx == 9 {
		p.PrayerOff(0)
		p.PrayerOff(3)
		p.PrayerOff(9)
	}
	if idx == 1 || idx == 4 || idx == 10 {
		p.PrayerOff(1)
		p.PrayerOff(4)
		p.PrayerOff(10)
	}
	if idx == 2 || idx == 5 || idx == 11 {
		p.PrayerOff(2)
		p.PrayerOff(5)
		p.PrayerOff(11)
	}
	p.TransAttrs.SetVar("prayer"+strconv.Itoa(idx), true)
}

func (p *Player) PrayerOff(idx int) {
	p.TransAttrs.SetVar("prayer"+strconv.Itoa(idx), false)
}

func (p *Player) SendPrayers() {
	p.SendPacket(PrayerStatus(p))
}

func (p *Player) Skulled() bool {
	return p.Attributes.VarInt("skullTime", 0) > 0
}

func (p *Player) SetSkulled(val bool) {
	if val {
		p.Attributes.SetVar("skullTime", TicksTwentyMin)
	} else {
		p.Attributes.UnsetVar("skullTime")
	}
	p.UpdateAppearance()
}

func (p *Player) StartCombat(target MobileEntity) {
	if p1, ok := target.(*Player); ok {
		p1.PlaySound("underattack")
		if !p.IsDueling() {
			p.SetSkulled(true)
		}
	}
	target.SetRegionRemoved()
	p.Teleport(target.X(), target.Y())
	p.AddState(MSFighting)
	target.AddState(MSFighting)
	p.SetDirection(RightFighting)
	target.SetDirection(LeftFighting)
	p.Transients().SetVar("fightTarget", target)
	target.Transients().SetVar("fightTarget", p)
	curRound := 0
	curTick := 0
	p.Tickables = append(p.Tickables, func() bool {
		curTick++
		if p1, ok := target.(*Player); ok {
			if !p1.Connected() {
				if p.HasState(MSFighting) {
					p.ResetFighting()
				}
				if p1.HasState(MSFighting) {
					p1.ResetFighting()
				}
				return true
			}
		}
		if !target.HasState(MSFighting) || !p.HasState(MSFighting) || !p.Connected() {
			if target.HasState(MSFighting) {
				target.ResetFighting()
			}
			if p.HasState(MSFighting) {
				p.ResetFighting()
			}
			return true
		}
		if curTick%2 == 0 {
			return false
		}
		var attacker, defender MobileEntity
		if curRound%2 == 0 {
			attacker = p
			defender = target
		} else {
			attacker = target
			defender = p
		}
		if _, ok := attacker.(*NPC); ok && p.PrayerActivated(12) {
			attacker.Transients().IncVar("fightRound", 1)
			curRound++
			return false
		}
		nextHit := int(math.Min(float64(defender.Skills().Current(StatHits)), float64(attacker.MeleeDamage(defender))))
		if attPlayer, ok := attacker.(*Player); ok {
			if nextHit > 0 {
				attPlayer.PlaySound("combat1b") // hit
			} else {
				attPlayer.PlaySound("combat1a") // miss
			}
		}
		if defPlayer, ok := defender.(*Player); ok {
			if nextHit > 0 {
				defPlayer.PlaySound("combat1b") // hit
			} else {
				defPlayer.PlaySound("combat1a") // miss
			}
		}
		// TODO: combat(2/3)(a/b) 2nd set is armor sound 3rd is ghostly undead sound
		defender.Skills().DecreaseCur(StatHits, nextHit)
		if defender.Skills().Current(StatHits) <= 0 {
			defender.Killed(attacker)
			return true
		}

		defender.Damage(nextHit)
		attacker.Transients().IncVar("fightRound", 1)
		curRound++
		return false
	})
}

//Killed kills this player, dropping all of its items where it stands.
func (p *Player) Killed(killer MobileEntity) {
	p.Transients().SetVar("deathTime", time.Now())
	p.PlaySound("death")
	p.SendPacket(Death)
	for i := 0; i < 18; i++ {
		p.Skills().SetCur(i, p.Skills().Maximum(i))
	}
	p.SendStats()
	p.SetDirection(North)

	var deathItems *Inventory

	if !p.IsDueling() {
		keepCount := 0
		if p.PrayerActivated(8) {
			// protect item prayer
			keepCount++
		}
		if !p.Skulled() {
			keepCount += 3
		}
		deathItems = p.Inventory.DeathDrops(keepCount)
	} else {
		deathItems = p.DuelOffer
	}
	var itemOwner *Player
	if killer, ok := killer.(*Player); killer != nil && ok {
		killer.DistributeMeleeExp(int(math.Ceil(MeleeExperience(p) / 4.0)))
		killer.Message("You have defeated " + p.Username() + "!")
		itemOwner = killer
	}
	deathItems.Range(func(item *Item) bool {
		if itemOwner == nil {
			AddItem(NewGroundItem(item.ID, item.Amount, p.X(), p.Y()))
		} else {
			AddItem(NewGroundItemFor(itemOwner.UsernameHash(), item.ID, item.Amount, p.X(), p.Y()))
		}
		return true
	})
	p.Inventory.RemoveAll(deathItems)
	for i := 0; i < 14; i++ {
		p.PrayerOff(i)
	}
	if itemOwner == nil {
		AddItem(NewGroundItem(20, 1, p.X(), p.Y()))
	} else {
		AddItem(NewGroundItemFor(itemOwner.UsernameHash(), 20, 1, p.X(), p.Y()))
	}

	if p.IsDueling() {
		if p.DuelTarget() != nil {
			p.DuelTarget().ResetDuel()
		}
	}
	p.ResetDuel()
	p.SetSkulled(false)
	p.SendPrayers()
	p.SendEquipBonuses()
	p.ResetFighting()
	plane := p.Plane()
	p.SetLocation(SpawnPoint, true)
	if p.Plane() != plane {
		p.SendPlane()
	}
}

func (p *Player) NpcWithin(id int, rad int) *NPC {
	p.LocalNPCs.lock.RLock()
	defer p.LocalNPCs.lock.RUnlock()
	var npc *NPC
	dist := math.MaxInt32
	for _, n := range p.LocalNPCs.set {
		if n := n.(*NPC); n.ID == id && n.WithinRange(p.Location, rad) {
			if d := n.LongestDelta(p.Location); d < dist {
				dist = d
				npc = n
			}
		}
	}

	return npc
}

//SendPlane sends the current plane of this player.
func (p *Player) SendPlane() {
	p.SendPacket(PlaneInfo(p))
}

//SendEquipBonuses sends the current equipment bonuses of this player.
func (p *Player) SendEquipBonuses() {
	p.SendPacket(EquipmentStats(p))
}

//Damage sends a player damage bubble for this player to itself and any nearby players.
func (p *Player) Damage(amt int) {
	for _, player := range p.NearbyPlayers() {
		player.SendPacket(PlayerDamage(p, amt))
	}
	p.SendPacket(PlayerDamage(p, amt))
}

//ItemBubble sends an item action bubble for this player to itself and any nearby players.
func (p *Player) ItemBubble(id int) {
	for _, player := range p.NearbyPlayers() {
		player.SendPacket(PlayerItemBubble(p, id))
	}
	p.SendPacket(PlayerItemBubble(p, id))
}

//SetStat sets the current, maximum, and experience levels of the skill at idx to lvl, and updates the client about it.
func (p *Player) SetStat(idx, lvl int) {
	p.Skills().SetCur(idx, lvl)
	p.Skills().SetMax(idx, lvl)
	p.Skills().SetExp(idx, LevelToExperience(lvl))
	p.SendStat(idx)
}

func (p *Player) CurrentShop() *Shop {
	s, ok := p.TransAttrs.Var("shop")
	if ok {
		if s, ok := s.(*Shop); ok && s != nil {
			return s
		}
	}
	return nil
}

//OpenBank opens a shop screen for the player and sets the appropriate state variables.
func (p *Player) OpenShop(shop *Shop) {
	if p.IsFighting() || p.IsTrading() || p.HasState(MSShopping) {
		return
	}
	p.AddState(MSShopping)
	p.Transients().SetVar("shop", shop)
	p.SendPacket(ShopOpen(shop))
}

//CloseBank closes the bank screen for this player and sets the appropriate state variables
func (p *Player) CloseShop() {
	if !p.HasState(MSShopping) {
		return
	}
	p.RemoveState(MSShopping)
	p.Transients().UnsetVar("shop")
	p.SendPacket(ShopClose)
}

//OpenBank opens a bank screen for the player and sets the appropriate state variables.
func (p *Player) OpenBank() {
	if p.IsFighting() || p.IsTrading() || p.HasState(MSBanking) {
		return
	}
	p.AddState(MSBanking)
	p.SendPacket(BankOpen(p))
}

//CloseBank closes the bank screen for this player and sets the appropriate state variables
func (p *Player) CloseBank() {
	if !p.HasState(MSBanking) {
		return
	}
	p.RemoveState(MSBanking)
	p.SendPacket(BankClose)
}

//SendUpdateTimer sends a system update countdown timer to the client.
func (p *Player) SendUpdateTimer() {
	p.SendPacket(SystemUpdate(int(time.Until(UpdateTime).Seconds())))
}

func (p *Player) SendMessageBox(msg string, big bool) {
	if big {
		p.SendPacket(BigInformationBox(msg))
	} else {
		p.SendPacket(InformationBox(msg))
	}
}

func (p *Player) SetCache(name string, val interface{}) {
	p.Attributes.SetVar(name, val)
}

func (p *Player) RemoveCache(name string) {
	p.Attributes.UnsetVar(name)
}

func (p *Player) Cache(name string) interface{} {
	v, ok := p.Attributes.Var(name)
	if ok {
		return v
	}
	return int64(0)
}

func (p *Player) OpenSleepScreen() {
	p.AddState(MSSleeping)
	p.SendPacket(SleepWord(p))
}