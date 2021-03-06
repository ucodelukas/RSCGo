/*
 * Copyright (c) 2020 Zachariah Knight <aeros.storkpk@gmail.com>
 *
 * Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 *
 */

package main

import (
	"crypto/tls"
	stdnet "net"
	"os"
	"sync"
	"strconv"
	"time"
	"strings"
	"math"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/jessevdk/go-flags"
	"github.com/BurntSushi/toml"

	"github.com/spkaeros/rscgo/pkg/crypto"
	"github.com/spkaeros/rscgo/pkg/config"
	"github.com/spkaeros/rscgo/pkg/definitions"
	"github.com/spkaeros/rscgo/pkg/tasks"
	rscerrors "github.com/spkaeros/rscgo/pkg/errors"
	"github.com/spkaeros/rscgo/pkg/db"
	"github.com/spkaeros/rscgo/pkg/isaac"
	"github.com/spkaeros/rscgo/pkg/xtea"
	"github.com/spkaeros/rscgo/pkg/rsa"
	"github.com/spkaeros/rscgo/pkg/rand"
	"github.com/spkaeros/rscgo/pkg/strutil"
	"github.com/spkaeros/rscgo/pkg/game/net"
	"github.com/spkaeros/rscgo/pkg/game"
	"github.com/spkaeros/rscgo/pkg/game/net/handshake"
	"github.com/spkaeros/rscgo/pkg/game/world"
	"github.com/spkaeros/rscgo/pkg/log"
	
	_ "github.com/spkaeros/rscgo/pkg/game/net/handlers"
)

const (
	TickMillis = time.Millisecond*640
)
//run Helper function for concurrently running a bunch of functions and waiting for them to complete
func run(fns ...func()) {
	w := &sync.WaitGroup{}
	do := func(fn func()) {
		w.Add(1)
		go func(fn func()) {
			defer w.Done()
			fn()
		}(fn)
	}

	for _, fn := range fns {
		do(fn)
	}
	w.Wait()
}

type (
	Flags struct {
		Verbose   []bool `short:"v" long:"verbose" description:"Display more verbose output"`
		Port      int    `short:"p" long:"port" description:"The TCP port for the game to listen on, (Websocket will use the port directly above it)"`
		Config    string `short:"c" long:"config" description:"Specify the TOML configuration file to load game settings from" default:"config.toml"`
		UseCipher bool   `short:"e" long:"encryption" description:"Enable command opcode encryption using a variant of ISAAC to encrypt net opcodes."`
	}
	Server struct {
		port        int
		*time.Ticker
	}
)

var (
	cliFlags = &Flags{}
	start = time.Now()
	newPlayers chan *world.Player
	tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{
			check(tls.LoadX509KeyPair("./data/ssl/fullchain.pem", "./data/ssl/privkey.pem")).(tls.Certificate),
		},
		ServerName: "rsclassic.dev",
		InsecureSkipVerify: false,
		// SessionTicketsDisabled: true,
		PreferServerCipherSuites: true,
		// ClientAuth: tls.RequireAndVerifyClientCert,
		// ClientAuth: tls.NoClientCert,
		// Rand: crand.Reader,
	}
	wsUpgrader = ws.Upgrader{
		Protocol: func(protocol []byte) bool {
			return string(protocol) == "binary"
		},
		ReadBufferSize:  5000,
		WriteBufferSize: 5000,
	}
)

func main() {
	if check(flags.Parse(cliFlags)) == nil {
		log.Warn("Error parsing command arguments!")
		os.Exit(1)
		return
	}
	if len(cliFlags.Config) == 0 {
		// Default to config.toml for config file
		cliFlags.Config = "config.toml"
	}

	config.TomlConfig.MaxPlayers = 1250
	config.TomlConfig.DataDir = "./data/"
	config.TomlConfig.DbioDefs = config.TomlConfig.DataDir + "dbio.conf"
	config.TomlConfig.PacketHandlerFile = config.TomlConfig.DataDir + "packets.toml"
	config.TomlConfig.Crypto.HashComplexity = 15
	config.TomlConfig.Crypto.HashLength = 32
	config.TomlConfig.Crypto.HashMemory = 8
	config.TomlConfig.Crypto.HashSalt = "rscgo./GOLANG!RULES/.1994"
	config.TomlConfig.Version = 235
	config.TomlConfig.Port = 43594 // +1 for websockets
	// TODO: data backend default to JSON or BSON maybe?
	config.TomlConfig.Database.PlayerDriver = "sqlite3"
	config.TomlConfig.Database.WorldDriver = "sqlite3"
	config.TomlConfig.Database.PlayerDB = "file:./data/players.db"
	config.TomlConfig.Database.WorldDB = "file:./data/world.db"
	if _, err := toml.DecodeFile(cliFlags.Config, &config.TomlConfig); err != nil {
		log.Fatal("Error decoding server config (file:%s):", err)
		os.Exit(2)
		return
	}
	if _, err := toml.DecodeFile(config.TomlConfig.DbioDefs, &config.TomlConfig.Database); err != nil {
		log.Fatal("Error decoding database i/o config (file:"+config.TomlConfig.DbioDefs+"):", err)
		os.Exit(3)
		return
	}
	run(db.ConnectEntityService, func() {
		db.DefaultPlayerService = db.NewPlayerServiceSql()
	}, func() {
		world.DefaultPlayerService = db.NewPlayerServiceSql()
	})
	if cliFlags.Port > 0 {
		config.TomlConfig.Port = cliFlags.Port
	}
	if config.Port() >= 65534 || config.Port() < 0 {
		log.Warn("Error: Invalid port number specified.")
		log.Warn("Valid port numbers are 1-65533 (needs the port 1 above it open to bind a websockets listener).")
		return 
	}
	config.Verbosity = int(math.Min(math.Max(float64(len(cliFlags.Verbose)), 0), 4))
	// Three init phases after data backend is connected--Entity definitions, then tile collision bitmask loading, followed by entity spawn locations
	// So, the order here of these three phases is important.  If you attempt to load object spawn locations during the same phase as the collision
	// data, it will result in a world filled with objects that are not solid.  Many similar bugs possible.  Best just to leave this be.
	run(db.LoadTileDefinitions, db.LoadObjectDefinitions, db.LoadBoundaryDefinitions, db.LoadItemDefinitions, db.LoadNpcDefinitions)
	run(world.LoadCollisionData, game.UnmarshalPackets, world.RunScripts)
	run(db.LoadObjectLocations, db.LoadNpcLocations, db.LoadItemLocations)

	if config.Verbose() {
		log.Debug("Loaded collision data from", len(world.Sectors), "map sectors")
		log.Debug("Loaded", len(definitions.TileOverlays), "tile types")
		log.Debug("Loaded", game.PacketCount(), "packet types, with handlers for", game.HandlerCount(), "of them")
		log.Debug("Loaded", world.ItemIndexer.Load(), "items and", len(definitions.Items), "item types")
		log.Debug("Loaded", world.Npcs.Size(), "NPCs and", len(definitions.Npcs), "NPC types")
		scenary, boundary := 0, 0
		for _, v := range world.GetAllObjects() {
			if v.(*world.Object).Boundary {
				boundary++
			} else {
				scenary++
			}
		}
		log.Debug("Loaded", scenary, "scenary objects, and", len(definitions.ScenaryObjects), "scenary types.")
		log.Debug("Loaded", boundary, "boundary objects, and", len(definitions.BoundaryObjects), "boundary types")
		log.Debug("Loading all game entitys took:", time.Since(start).Seconds(), "seconds")
		if config.Verbosity >= 2 {
			log.Debugf("Triggers[\n\t%d item actions,\n\t%d scenary actions,\n\t%d boundary actions,\n\t%d npc actions,\n\t%d item->boundary actions,\n\t%d item->scenary actions,\n\t%d attacking NPC actions,\n\t%d killing NPC actions\n];\n", len(world.ItemTriggers), len(world.ObjectTriggers), len(world.BoundaryTriggers), len(world.NpcTriggers), len(world.InvOnBoundaryTriggers), len(world.InvOnObjectTriggers), len(world.NpcAtkTriggers), len(world.NpcDeathTriggers))
		}
	}
	log.Debug("Listening at TCP port " + strconv.Itoa(config.Port()))// + " (TCP), " + strconv.Itoa(config.WSPort()) + " (websockets)")
	log.Debug()
	log.Debug("RSCGo has finished initializing world; we hope you enjoy it")
	go Instance.Bind(config.Port())
	// go Instance.WsBind()
	Instance.Start()
}

func needsData(err error) bool {
	return err.Error() == "Socket buffer has less bytes available than we need to form a message packet."
}

var Instance = &Server{Ticker: time.NewTicker(world.TickMillis)}

func (s *Server) accept(l stdnet.Listener) *world.Player {
	socket, err := l.Accept()
	if err != nil {
		log.Warn("Problem accepting incoming TLS connection from '" + socket.RemoteAddr().String() + "':", err)
		return nil
	}
	if check(wsUpgrader.Upgrade(socket)) == nil {
		log.Debug("could not upgrade to websocket")
		// return nil
	}
	p := world.NewPlayer(socket)
	p.Websocket = true
	p.Writer = wsutil.NewWriter(p.Socket, ws.StateServerSide, ws.OpBinary)
	// log.Debug(p.Socket)
	return p
}

func (s *Server) Bind(port int) bool {
	// listener := check(tls.Listen("tcp", ":"+strconv.Itoa(s.port), tlsConfig)).(stdnet.Listener)
	listener := tls.NewListener(check(stdnet.Listen("tcp", ":" + strconv.Itoa(port))).(stdnet.Listener), tlsConfig)
new_plr:
	for {
		player := s.accept(listener)
again:
		login, err := player.ReadPacket()
		if err != nil {
			if needsData(err) {
				goto again
			} else if err.(rscerrors.NetError).Fatal {
				player.Socket.Close()
				continue
			}
		}
		if login == nil {
			goto again
		}
		sendReply := func(i handshake.ResponseCode, reason string) {
			player.Writer.Write([]byte{byte(i)})
			player.Writer.Flush()
			if !i.IsValid() {
				close(player.InQueue)
				close(player.OutQueue)
				log.Debug("[LOGIN]", player.Username() + "@" + player.CurrentIP(), "failed to login (" + reason + ")")
				player.Destroy()
			}
		}

		if login.Opcode == 0 {
			if !world.UpdateTime.IsZero() {
				sendReply(handshake.ResponseServerRejection, "System update in progress")
				continue new_plr
			}
			if world.Players.Size() >= config.MaxPlayers() {
				sendReply(handshake.ResponseWorldFull, "Out of usable player slots")
				continue new_plr
			}
			if handshake.LoginThrottle.Recent(player.CurrentIP(), time.Second*10) >= 5 {
				sendReply(handshake.ResponseSpamTimeout, "Too many recent invalid login attempts (5 in 10 seconds)")
				continue new_plr
			}

			player.SetReconnecting(login.ReadBoolean())
			if ver := login.ReadUint32(); ver != config.Version() {
				sendReply(handshake.ResponseUpdated, "Invalid client version (" + strconv.Itoa(ver) + ")")
				continue new_plr
			}

			rsaSize := login.ReadUint16()
			data := make([]byte, rsaSize)
			rsaRead := login.Read(data)
			if rsaRead < rsaSize {
				log.Debug("short RSA block")
				player.Writer.Write([]byte{byte(handshake.ResponseServerRejection)})
				player.Writer.Flush()
				continue
			}

			rsaBlock := net.NewPacket(0, rsa.RsaKeyPair.Decrypt(data))
			checksum := rsaBlock.ReadUint8()
			// It's been suggested to me that this first byte assures us that the RSA block could decode properly,
			// it's only wrong for this purpose a statistically insignificant amount of time.  >99% accurate, as I understand it.
			if checksum != 10 {
				log.Debug("Bad checksum:", checksum)
				player.Writer.Write([]byte{byte(handshake.ResponseServerRejection)})
				player.Writer.Flush()
				continue
			}
			var keys = make([]int, 4)
			for i := range keys {
				keys[i] = rsaBlock.ReadUint32()
			}
			player.OpCiphers[0] = isaac.New(keys...)
			player.OpCiphers[1] = isaac.New(keys...)
			password := strings.TrimSpace(rsaBlock.ReadString())
			// The rscplus team viewed this data below as a nonce, but in my opinion, this is not the motivation for this data.
			// I'd call these more of an initialization vector (IV), as wikipedia defines it, used to make RSA semantically secure.
			rsaBlock.Skip(8)
			blockSize := login.ReadUint16()
			var block = make([]byte, blockSize)
			if login.Available() != blockSize {
				log.Debug("XTEA block size recv'd doesn't take up the rest of the packets available buffer size! (it should)")
				log.Debugf("\t{ blockSize:%d, login.Available():%d }\n", blockSize, login.Available())
			}
			login.Read(block)
			usernameData := xtea.New(keys).Decrypt(block)
			// first byte of this block is limit30 parameter from the game client applet; boolean, use unknown
			// I suppose the next 24 bytes are to ensure the stream gets sufficiently shuffled in each packet, preventing identifying markers appearing
			// finally, the null-terminated UTF-8 encoded username comes at offset 25 and beyond.
			username := string(usernameData[25:])
			player.SetVar("username", strutil.Base37.Encode(username))
			if world.Players.ContainsHash(player.UsernameHash()) {
				sendReply(handshake.ResponseLoggedIn, "Player with same username is already logged in")
				continue new_plr
			}
			var dataService = db.DefaultPlayerService
			if !dataService.PlayerNameExists(player.Username()) || !dataService.PlayerValidLogin(player.UsernameHash(), crypto.Hash(password)) {
				handshake.LoginThrottle.Add(player.CurrentIP())
				sendReply(handshake.ResponseBadPassword, "Invalid credentials")
				continue new_plr
			}
			if !dataService.PlayerLoad(player) {
				sendReply(handshake.ResponseDecodeFailure, "Could not load player profile; is the dataService setup properly?")
				continue new_plr
			}

			if player.Reconnecting() {
				sendReply(handshake.ResponseReconnected, "")
				continue new_plr
			}
			switch player.Rank() {
			case 2:
				sendReply(handshake.ResponseAdministrator|handshake.ResponseLoginAcceptBit, "")
			case 1:
				sendReply(handshake.ResponseModerator|handshake.ResponseLoginAcceptBit, "")
			default:
				sendReply(handshake.ResponseLoginSuccess|handshake.ResponseLoginAcceptBit, "")
			}
			go func() {
				defer player.Destroy()
				defer func() {
					// makes the queue goroutines kill themself
					player.InQueue <- nil
					player.OutQueue <- nil
				}()
				for {
					packet, err := player.ReadPacket()
					if err != nil || packet == nil {
						if needsData(err) {
							continue
						} else if err.(rscerrors.NetError).Fatal {
							player.Socket.Close()
							return
						}
					}
					player.InQueue <- packet
				}
			}()
			go func() {
				defer close(player.InQueue)
				for {
					select {
					case packet, ok := <-player.InQueue:
						if packet == nil || !ok {
							return
						}
						// script packet handlers are the most `modern` solution, and will be the default selected for any incoming packet
						if handlePacket := world.PacketTriggers[packet.Opcode]; handlePacket != nil {
							handlePacket(player, packet)
							continue
						}
						if handlePacket := game.Handler(packet.Opcode); handlePacket != nil {
							// This is old legacy go code handlers that are deprecated and being replaced with the aforementioned scripting API
							handlePacket(player, packet)
						}
					}
				}
			}()
			go func() {
				defer close(player.OutQueue)
				defer player.WriteNow(*world.Logout)
				for {
					select {
					case packet, ok := <-player.OutQueue:
						if packet == nil || !ok {
							return
						}
						player.WriteNow(*packet)
					}
				}
			}()
			log.Debug("[LOGIN]", player.Username() + "@" + player.CurrentIP(), "successfully logged in")
			player.Initialize()
			continue new_plr
		}
	}
	return false
}


func (s *Server) Start() {
	defer s.Ticker.Stop()
	wait := sync.WaitGroup{}
	for range s.C {
		tasks.TickList.Call(nil)
		world.Players.AsyncRange(func(p *world.Player) {
			if p == nil {
				return
			}
			if fn := p.TickAction(); fn != nil && !fn() {
				p.ResetTickAction()
			}
			p.Tickables.Call(interface{}(p))

			p.TraversePath()
		})
		wait.Add(world.Npcs.Size())
		world.Npcs.RangeNpcs(func(n *world.NPC) bool {
			go func() {
				defer wait.Done()
				if n.Busy() || n.IsFighting() {
					return
				}
				if world.Chance(25) && n.VarInt("steps", 0) <= 0 && n.VarInt("ticks", 0) <= 0 {
					// move some amount between 2-15 tiles, moving 1 tile per tick
					n.SetVar("steps", rand.Intn(13+1)+2)
					// wait some amount between 25-50 ticks before doing this again
					n.SetVar("ticks", rand.Intn(25+1)+25)
				}
				if n.VarInt("ticks", 0) > 0 {
					n.Dec("ticks", 1)
				}
				// wander aimlessly until we run out of scheduled steps
				if n.VarInt("steps", 0) > 0 {
					n.TraversePath()
				}
			}()
			return false
		})
		wait.Wait()
		world.Players.AsyncRange(func(p *world.Player) {
			if p == nil {
				return
			}
			positions := world.PlayerPositions(p)
			if positions != nil {
				p.WritePacket(positions)
			}
			npcUpdates := world.NPCPositions(p)
			if npcUpdates != nil {
				p.WritePacket(npcUpdates)
			}
			appearances := world.PlayerAppearances(p)
			if appearances != nil {
				p.WritePacket(appearances)
			}
			npcEvents := world.NpcEvents(p)
			if npcEvents != nil {
				p.WritePacket(npcEvents)
			}
			objectUpdates := world.ObjectLocations(p)
			if objectUpdates != nil {
				p.WritePacket(objectUpdates)
			}
			boundaryUpdates := world.BoundaryLocations(p)
			if boundaryUpdates != nil {
				p.WritePacket(boundaryUpdates)
			}
			itemUpdates := world.ItemLocations(p)
			if itemUpdates != nil {
				p.WritePacket(itemUpdates)
			}
			clearDistantChunks := world.ClearDistantChunks(p)
			if clearDistantChunks != nil {
				p.WritePacket(clearDistantChunks)
			}
		})

		world.Players.AsyncRange(func(p *world.Player) {
			if p == nil {
				return
			}
			p.PostTickables.Call(interface{}(p))
			p.ResetRegionRemoved()
			p.ResetRegionMoved()
			p.ResetSpriteUpdated()
			p.ResetAppearanceChanged()
		})
		world.ResetNpcUpdateFlags()
		world.Ticks.Inc()
	}
}

//Stop This will stop the game instance, if it is running.
func (s *Server) Stop() {
	log.Debug("Stopping...")
	os.Exit(0)
}

func check(i interface{}, err error) interface{} {
	if err != nil {
		log.Debug("Error encountered:", err)
		return nil
	}
	return i
}
