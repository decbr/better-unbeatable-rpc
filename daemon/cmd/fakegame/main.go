package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

type step struct {
	name string
	hold time.Duration
	json string
}

func main() {
	port := flag.Int("port", 47820, "daemon UDP port")
	once := flag.String("once", "", "send this single JSON message and exit")
	stepLen := flag.Duration("step", 8*time.Second, "how long to hold each scripted state")
	hold := flag.String("hold", "", "send this one state until Ctrl+C: "+strings.Join(holdNames(), ", "))
	flag.Parse()

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: *port})
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	send := func(s string) {
		if _, err := conn.Write([]byte(s)); err != nil {
			log.Printf("send: %v", err)
		}
	}

	if *once != "" {
		send(*once)
		return
	}

	// like the plugin, every message carries when the "game" was launched
	launched := time.Now().Unix()
	stamp := func(msg string) string {
		return strings.Replace(msg, `{"v":1,`, fmt.Sprintf(`{"v":1,"launchedAt":%d,`, launched), 1)
	}

	if *hold != "" {
		msg, ok := holds[*hold]
		if !ok {
			log.Fatalf("unknown -hold %q (have: %s)", *hold, strings.Join(holdNames(), ", "))
		}
		msg = stamp(msg)
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		log.Printf("holding %s; Ctrl+C to stop", *hold)
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for send(msg); ; send(msg) {
			select {
			case <-stop:
				send(`{"v":1,"state":"closed"}`)
				log.Printf("closed")
				return
			case <-t.C:
			}
		}
	}

	steps := []step{
		{"idle (menus, stars)", *stepLen, `{"v":1,"state":"idle","stars":12.345}`},
		{"arcade song", *stepLen, `{"v":1,"state":"arcade",` + song + `}`},
		{"paused", *stepLen, `{"v":1,"state":"arcade",` + song + `,"paused":true}`},
		{"resumed", *stepLen, `{"v":1,"state":"arcade",` + song + `}`},
		{"results (new PB)", *stepLen, `{"v":1,"state":"results","song":"RE-TUNED","artist":"peak divide ft. Rachel Lake","charter":"charter","difficulty":"UNBEATABLE","level":20,"highScore":1201234,"accuracy":0.9512,"rank":"S+","stars":12.401}`},
		{"story mode", *stepLen, `{"v":1,"state":"story","chapter":"Chapter 4","stars":12.401}`},
		{"idle again", *stepLen, `{"v":1,"state":"idle","stars":12.401}`},
		{"closed", 0, `{"v":1,"state":"closed"}`},
	}

	for _, st := range steps {
		log.Printf("-> %s", st.name)
		msg := stamp(st.json)
		send(msg)
		deadline := time.Now().Add(st.hold)
		for time.Now().Before(deadline) {
			time.Sleep(2 * time.Second)
			send(msg)
		}
	}
	log.Printf("done")
}

const song = `"song":"RE-TUNED","artist":"peak divide ft. Rachel Lake","charter":"charter","difficulty":"UNBEATABLE","level":20,"highScore":1166540,"accuracy":0.9361,"rank":"S","stars":12.345`

// holds are the states -hold can send.
var holds = map[string]string{
	"idle":    `{"v":1,"state":"idle","stars":12.345}`,
	"arcade":  `{"v":1,"state":"arcade",` + song + `}`,
	"paused":  `{"v":1,"state":"arcade",` + song + `,"paused":true}`,
	"results": `{"v":1,"state":"results",` + song + `}`,
	"story":   `{"v":1,"state":"story","chapter":"Chapter 4","stars":12.345}`,
}

func holdNames() []string {
	names := make([]string, 0, len(holds))
	for n := range holds {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
