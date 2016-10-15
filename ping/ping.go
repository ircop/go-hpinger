package ping

import (
	"fmt"
	"net"
	"os"
	"time"
	
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const ProtocolICMP = 1



type Host struct {
	Ip		string
	Alive		bool
	OldAlive	bool
	OldAliveStr	string
	Id		string
	ObjValue	string
	Model		string
	Addr		string
}

type Pinger struct {
	Hosts		map[string]Host
}

func NewPinger() *Pinger {
	return &Pinger{
		Hosts: make(map[string]Host),
	}
}

func (p *Pinger) Init() {
	fmt.Printf("TEST PRINTF INIT\n")
	p.Hosts = make(map[string]Host)
}

func (p *Pinger) AddHost(host Host) {
	p.Hosts[host.Ip] = host
}

func (p *Pinger) Run() (map[string]Host) {
	
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		fmt.Printf("Error listening: %s\n", err)
		return nil
	}
	defer c.Close()
	
	// msg to write
	wm := icmp.Message {
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte("HELO"),
		},
	}
	
	// write buf
	wb, err := wm.Marshal(nil)
	if err != nil {
		fmt.Printf("Error wm.Marshal: %s\n", err)
		return nil
	}
	
//	go p.SendEchos( wb, c )
	/*
	for _, H := range p.Hosts {
//		fmt.Printf("echo request to %s\n", H.Ip)
		// send req.
		if _, err := c.WriteTo(wb, &net.IPAddr{IP: net.ParseIP(H.Ip)}); err != nil {
			fmt.Printf("Error writing to buffer (%s): %s\n", H.Ip, err)
			continue
		}
	}
	*/
	
	// read buf
	rb := make([]byte, 1500)

	ch := make(chan int)
	
	go func() {
	PingLoop:
	for {
		select {
			
			case sig := <-ch:
				fmt.Printf("sig %d received, breaking\n", sig)
				break PingLoop
			default:
			
			n, peer, err := c.ReadFrom(rb)
			if err != nil {
				fmt.Printf("Error reading buffer: %s\n", err)
				return
			}
		
			// check expecting ipaddr
			host, ok := p.Hosts[peer.String()]
			if !ok {
				continue
			}
		
			rm, err := icmp.ParseMessage(ProtocolICMP, rb[:n])
			if err != nil {
				fmt.Printf("Error parsing message: %s\n", err)
				continue
			}
		
		
			pingOk := false
			switch rm.Type {
				case ipv4.ICMPTypeEchoReply:
					pingOk = true
				default:
					pingOk = false
			}
		
			if( pingOk == true ) {
				host.Alive = true
				p.Hosts[peer.String()] = host
			} else {
				host.Alive = false
				p.Hosts[peer.String()] = host
			
				if( host.OldAlive == true ) {
					// send request again
					go func() {
						if _, err := c.WriteTo(wb, &net.IPAddr{IP: net.ParseIP(host.Ip)}); err != nil {
							fmt.Printf("Error writing to buffer (%s): %s\n", host.Ip, err)
						}
					}()
				}
			}
		}
	}
	}()
	
	p.SendEchos( wb, c )
	time.Sleep( 2 * time.Second )
	p.SendEchos( wb, c )
	
	time.Sleep( 5 * time.Second )
	ch <- 1
	close(ch)
	
	DiffHosts := make(map[string]Host)
	
	for _, H := range p.Hosts {
		if( H.Alive != H.OldAlive ) {
			DiffHosts[H.Ip] = H
		}
	}
	
	return DiffHosts
}

func (p *Pinger) SendEchos(wb []byte, c *icmp.PacketConn) {
	for _, H := range p.Hosts {
//		fmt.Printf("echo request to %s\n", H.Ip)
		// send req.
		if _, err := c.WriteTo(wb, &net.IPAddr{IP: net.ParseIP(H.Ip)}); err != nil {
			fmt.Printf("Error writing to buffer (%s): %s\n", H.Ip, err)
			continue
		}
		time.Sleep( 10 * time.Millisecond )
	}
}

func (p *Pinger) Clear() {
	p.Hosts = make(map[string]Host)
}
