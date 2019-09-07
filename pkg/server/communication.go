package server

import (
	"bitbucket.org/zlacki/rscgo/pkg/entity"
	"bitbucket.org/zlacki/rscgo/pkg/server/packets"
	"bitbucket.org/zlacki/rscgo/pkg/strutil"
)

func init() {
	PacketHandlers["chatmsg"] = func(c *Client, p *packets.Packet) {
		for _, v := range c.player.LocalPlayers.List {
			if v, ok := v.(*entity.Player); ok && (!v.ChatBlocked() || v.FriendsWith(c.player.UserBase37)) {
				c1 := ClientFromIndex(v.Index)
				if c1 != nil {
					c1.outgoingPackets <- packets.PlayerChat(c.Index, string(p.Payload))
				}
			}
		}
	}
	PacketHandlers["addfriend"] = func(c *Client, p *packets.Packet) {
		hash := p.ReadLong()
		defer func() {
			c.outgoingPackets <- packets.FriendList(c.player)
		}()
		if c.player.FriendsWith(hash) {
			c.outgoingPackets <- packets.ServerMessage("@que@You are already friends with that person!")
			return
		}
		if c.player.Ignored(hash) {
			c.outgoingPackets <- packets.ServerMessage("@que@Please remove '" + strutil.DecodeBase37(hash) + "' from your ignore list before friending them.")
			return
		}
		if c1, ok := Clients[hash]; ok && c.player.FriendBlocked() {
			c1.outgoingPackets <- packets.FriendUpdate(c.player.UserBase37, true)
		}
		c.player.FriendList[hash] = ClientFromHash(hash) != nil
		//		c.player.FriendList = append(c.player.FriendList, hash)
	}
	PacketHandlers["removefriend"] = func(c *Client, p *packets.Packet) {
		hash := p.ReadLong()
		defer func() {
			c.outgoingPackets <- packets.FriendList(c.player)
		}()
		if !c.player.FriendsWith(hash) {
			c.outgoingPackets <- packets.ServerMessage("@que@You are not friends with that person!")
			return
		}
		if c1, ok := Clients[hash]; ok && c.player.FriendBlocked() {
			c1.outgoingPackets <- packets.FriendUpdate(c.player.UserBase37, false)
		}
		delete(c.player.FriendList, hash)
	}
	PacketHandlers["addignore"] = func(c *Client, p *packets.Packet) {
		hash := p.ReadLong()
		defer func() {
			c.outgoingPackets <- packets.IgnoreList(c.player)
		}()
		if c.player.FriendsWith(hash) {
			c.outgoingPackets <- packets.ServerMessage("@que@Please remove '" + strutil.DecodeBase37(hash) + "' from your friend list before ignoring them.")
			return
		}
		if c.player.Ignored(hash) {
			c.outgoingPackets <- packets.ServerMessage("@que@You are already ignoring that person!")
			return
		}
		LogInfo.Println(hash)
		c.player.IgnoreList = append(c.player.IgnoreList, hash)
	}
	PacketHandlers["removeignore"] = func(c *Client, p *packets.Packet) {
		hash := p.ReadLong()
		defer func() {
			c.outgoingPackets <- packets.IgnoreList(c.player)
		}()
		if !c.player.Ignored(hash) {
			c.outgoingPackets <- packets.ServerMessage("@que@You are not ignoring that person!")
			return
		}
		for i, v := range c.player.IgnoreList {
			if v == hash {
				newSize := len(c.player.IgnoreList) - 1
				c.player.IgnoreList[i] = c.player.IgnoreList[newSize]
				c.player.IgnoreList = c.player.IgnoreList[:newSize]
				return
			}
		}
	}
	//	PacketHandlers[84] = func(c *Client, p *packets.Packet) {
	//		index, _ := p.ReadShort()
	//		c.player.Appearances = append(c.player.Appearances, int(index))
	//	}
}
