package packethandlers

import (
	"github.com/spkaeros/rscgo/pkg/server/clients"
	"github.com/spkaeros/rscgo/pkg/server/log"
	"github.com/spkaeros/rscgo/pkg/server/packet"
	"github.com/spkaeros/rscgo/pkg/server/packetbuilders"
	"github.com/spkaeros/rscgo/pkg/server/world"
	"time"
)

func init() {
	PacketHandlers["attacknpc"] = func(c clients.Client, p *packet.Packet) {
		npc := world.GetNpc(p.ReadShort())
		if npc == nil {
			log.Suspicious.Printf("player[%v] tried to attack nil NPC\n", c)
			return
		}
		if c.Player().State != world.MSIdle {
			return
		}
		if !world.NpcDefs[npc.ID].Attackable {
			log.Info.Println("Player attacked not attackable NPC!", world.NpcDefs[npc.ID])
			return
		}
		c.Player().SetDistancedAction(func() bool {
			if c.Player().NextTo(npc.Location) && c.Player().WithinRange(npc.Location, 1) {
				c.Player().ResetPath()
				npc.ResetPath()
				world.UpdateRegionMob(c.Player(), npc.CurX(), npc.CurY())
				c.Player().Teleport(npc.CurX(), npc.CurY())
				c.Player().State = world.MSFighting
				npc.State = world.MSFighting
				c.Player().SetDirection(world.LeftFighting)
				npc.SetDirection(world.RightFighting)
				c.Player().TransAttrs.SetVar("fighting", true)
				c.Player().TransAttrs.SetVar("fightTarget", npc)
				npc.TransAttrs.SetVar("fighting", true)
				npc.TransAttrs.SetVar("fightTarget", c.Player())
				go func() {
					ticker := time.NewTicker(time.Millisecond * 1200)
					defer ticker.Stop()
					curRound := 0
					for range ticker.C {
						if !c.Player().TransAttrs.VarBool("fighting", false) || !c.Player().TransAttrs.VarBool("connected", false) {
							if npc.TransAttrs.VarBool("fighting", false) {
								npc.TransAttrs.UnsetVar("fighting")
								npc.TransAttrs.UnsetVar("fightRound")
								npc.TransAttrs.UnsetVar("fightTarget")
								npc.State = world.MSIdle
								npc.SetDirection(world.North)
							}
							return
						}
						if curRound % 2 == 0 {
							attacker := c.Player()
							defender := npc
							nextHit := attacker.MeleeDamage(defender.Mob)
							if nextHit > defender.Skillset.Current(3) {
								nextHit = defender.Skillset.Current(3)
							}
							defender.Skillset.DecreaseCur(3, nextHit)
							if defender.Skillset.Current(3) <= 0 {
								world.UpdateRegionMob(c.Player(), world.DeathSpot.CurX(), world.DeathSpot.CurY())
								npc.Teleport(world.DeathSpot.CurX(), world.DeathSpot.CurY())
								go func() {
									time.Sleep(time.Second * 10)
									world.UpdateRegionMob(c.Player(), npc.StartPoint.CurX(), npc.StartPoint.CurY())
									npc.Teleport(npc.StartPoint.CurX(), npc.StartPoint.CurY())
									npc.Skillset.SetCur(3, npc.Skillset.Maximum(3))
								}()
								c.Player().ResetFighting()
								return
							}
							c.SendPacket(packetbuilders.NpcDamage(defender.Index, nextHit, defender.Skillset.Current(3), defender.Skillset.Maximum(3)))
							for _, p1 := range c.Player().NearbyPlayers() {
								p1.SendPacket(packetbuilders.NpcDamage(defender.Index, nextHit, defender.Skillset.Current(3), defender.Skillset.Maximum(3)))
							}

							attacker.TransAttrs.SetVar("fightRound", attacker.TransAttrs.VarInt("fightRound", 0) + 1)
						} else {
							attacker := npc
							defender := c.Player()
							nextHit := attacker.MeleeDamage(*defender.Mob)
							if nextHit > defender.Skillset.Current(3) {
								nextHit = defender.Skillset.Current(3)
							}
							defender.Skillset.DecreaseCur(3, nextHit)
							if defender.Skillset.Current(3) <= 0 {
								c.Player().ResetFighting()
								c.SendPacket(packetbuilders.Death)
								c.Player().Skillset.SetCur(3, c.Player().Skillset.Maximum(3))
								world.UpdateRegionMob(c.Player(), 220, 445)
								c.Player().Teleport(220, 445)
								return
							}
							c.SendPacket(packetbuilders.PlayerDamage(defender.Index, nextHit, defender.Skillset.Current(3), defender.Skillset.Maximum(3)))
							for _, p1 := range c.Player().NearbyPlayers() {
								p1.SendPacket(packetbuilders.PlayerDamage(defender.Index, nextHit, defender.Skillset.Current(3), defender.Skillset.Maximum(3)))
							}
							attacker.TransAttrs.SetVar("fightRound", attacker.TransAttrs.VarInt("fightRound", 0) + 1)
						}
						curRound++
					}
				}()
				return true
			} else {
				c.Player().SetPath(world.MakePath(c.Player().Location, npc.Location))
			}
			return false
		})
	}
}
