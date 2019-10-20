package world

import "sync"

//MobState Mob state.
type MobState uint8

const (
	//MSIdle The default MobState, means doing nothing.
	MSIdle MobState = iota
	//MSWalking The mob is walking.
	MSWalking
	//MSBanking The mob is banking.
	MSBanking
	//MSChatting The mob is chatting with a NPC
	MSChatting
	//MSMenuChoosing The mob is in a query menu
	MSMenuChoosing
	//MSTrading The mob is negotiating a trade.
	MSTrading
	//MSDueling The mob is negotiating a duel.
	MSDueling
	//MSFighting The mob is fighting.
	MSFighting
	//MSBatching The mob is performing a skill that repeats itself an arbitrary number of times.
	MSBatching
	//MSSleeping The mob is using a bed or sleeping bag, and trying to solve a CAPTCHA
	MSSleeping
)

//Mob Represents a mobile entity within the game world.
type Mob struct {
	Entity
	State      MobState
	Skillset   *SkillTable
	Path       *Pathway
	PathLock   sync.RWMutex
	TransAttrs *AttributeList
}

//Busy Returns true if this mobs state is anything other than idle. otherwise returns false.
func (m *Mob) Busy() bool {
	return m.State != MSIdle
}

//Direction Returns the mobs direction.
func (m *Mob) Direction() int {
	return m.TransAttrs.VarInt("direction", 0)
}

//SetDirection Sets the mobs direction.
func (m *Mob) SetDirection(direction int) {
	m.TransAttrs.SetVar("plrchanged", true)
	m.TransAttrs.SetVar("direction", direction)
}

//SetPath Sets the mob's current pathway to path.  If path is nil, effectively resets the mobs path.
func (m *Mob) SetPath(path *Pathway) {
	m.PathLock.Lock()
	m.Path = path
	m.PathLock.Unlock()
}

//ResetPath Sets the mobs path to nil, to stop the traversal of the path instantly
func (m *Mob) ResetPath() {
	m.PathLock.Lock()
	m.Path = nil
	m.PathLock.Unlock()
}

//TraversePath If the mob has a path, calling this method will change the mobs location to the next location described by said Path data structure.  This should be called no more than once per game tick.
func (m *Mob) TraversePath() {
	m.PathLock.RLock()
	path := m.Path
	m.PathLock.RUnlock()
	if path == nil {
		return
	}
	if m.AtLocation(path.Waypoint(path.CurrentWaypoint)) {
		path.CurrentWaypoint++
	}
	if m.FinishedPath() {
		m.ResetPath()
		return
	}
	m.TransAttrs.SetVar("plrmoved", true)
	m.SetLocation(path.NextTile(m.X, m.Y))
}

//FinishedPath Returns true if the mobs path is nil, the paths current waypoint exceeds the number of waypoints available, or the next tile in the path is not a valid location, implying that we have reached our destination.
func (m *Mob) FinishedPath() bool {
	m.PathLock.RLock()
	defer m.PathLock.RUnlock()
	if m.Path == nil {
		return true
	}
	return m.Path.CurrentWaypoint >= len(m.Path.WaypointsX) || !m.Path.NextTile(m.X, m.Y).WithinWorld()
}

//UpdateDirection Updates the direction the mob is facing based on where the mob is trying to move, and where the mob is currently at.
func (m *Mob) UpdateDirection(destX, destY int) {
	sprites := [3][3]int{{3, 2, 1}, {4, -1, 0}, {5, 6, 7}}
	xIndex := m.X - destX + 1
	yIndex := m.Y - destY + 1
	if xIndex >= 0 && yIndex >= 0 && xIndex < 3 && yIndex < 3 {
		m.SetDirection(sprites[xIndex][yIndex])
	} else {
		m.SetDirection(int(North))
	}
}

//SetLocation Sets the mobs location.
func (m *Mob) SetLocation(location *Location) {
	m.SetCoords(location.X, location.Y)
}

//SetCoords Sets the mobs locations coordinates.
func (m *Mob) SetCoords(x, y int) {
	m.UpdateDirection(x, y)
	m.lock.Lock()
	m.X = x
	m.Y = y
	m.lock.Unlock()
}

//AttrList A type alias for a map of strings to empty interfaces, to hold generic mob information for easy serialization and to provide dynamic insertion/deletion of new mob properties easily
type AttrList map[string]interface{}

//AttributeList A concurrency-safe collection data type for storing misc. variables by a descriptive name.
type AttributeList struct {
	Set  map[string]interface{}
	Lock sync.RWMutex
}

//Range Runs fn(key, value) for every entry in this attribute list.
func (attributes *AttributeList) Range(fn func(string, interface{})) {
	attributes.Lock.RLock()
	defer attributes.Lock.RUnlock()
	for k, v := range attributes.Set {
		fn(k, v)
	}
}

//SetVar Sets the attribute mapped at name to value in the attribute map.
func (attributes *AttributeList) SetVar(name string, value interface{}) {
	attributes.Lock.Lock()
	attributes.Set[name] = value
	attributes.Lock.Unlock()
}

//UnsetVar Removes the attribute with the key `name` from this attribute set.
func (attributes *AttributeList) UnsetVar(name string) {
	attributes.Lock.Lock()
	delete(attributes.Set, name)
	attributes.Lock.Unlock()
}

//VarInt If there is an attribute assigned to the specified name, returns it.  Otherwise, returns zero
func (attributes *AttributeList) VarInt(name string, zero int) int {
	attributes.Lock.RLock()
	defer attributes.Lock.RUnlock()
	if _, ok := attributes.Set[name].(int); !ok {
		return zero
	}

	return attributes.Set[name].(int)
}

//VarLong If there is an attribute assigned to the specified name, returns it.  Otherwise, returns zero
func (attributes *AttributeList) VarLong(name string, zero uint64) uint64 {
	attributes.Lock.RLock()
	defer attributes.Lock.RUnlock()
	if _, ok := attributes.Set[name].(uint64); !ok {
		return zero
	}

	return attributes.Set[name].(uint64)
}

//VarBool If there is an attribute assigned to the specified name, returns it.  Otherwise, returns zero
func (attributes *AttributeList) VarBool(name string, zero bool) bool {
	attributes.Lock.RLock()
	defer attributes.Lock.RUnlock()
	if _, ok := attributes.Set[name].(bool); !ok {
		return zero
	}

	return attributes.Set[name].(bool)
}

//AppearanceTable Represents a mobs appearance.
type AppearanceTable struct {
	Head   int
	Body   int
	Male   bool
	Hair   int
	Top    int
	Bottom int
	Skin   int
}

//NewAppearanceTable Returns a reference to a new appearance table with specified parameters
func NewAppearanceTable(head, body int, male bool, hair, top, bottom, skin int) *AppearanceTable {
	return &AppearanceTable{head, body, male, hair, top, bottom, skin}
}

//SkillTable Represents a skill table for a mob.
type SkillTable struct {
	Current    [18]int
	Maximum    [18]int
	Experience [18]int
}

//CombatLevel Calculates and returns the combat level for this skill table.
func (s *SkillTable) CombatLevel() int {
	aggressiveTotal := float32(s.Maximum[0] + s.Maximum[2])
	defensiveTotal := float32(s.Maximum[1] + s.Maximum[3])
	spiritualTotal := float32((s.Maximum[5] + s.Maximum[6]) / 8)
	ranged := float32(s.Maximum[4])
	if aggressiveTotal < ranged*1.5 {
		return int((defensiveTotal / 4) + (ranged * 0.375) + spiritualTotal)
	}
	return int((aggressiveTotal / 4) + (defensiveTotal / 4) + spiritualTotal)
}

//NPC Represents a single non-playable character within the game world.
type NPC struct {
	Mob
}
