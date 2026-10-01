package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

type step struct {
	name string
	hold time.Duration
	json func(now int64) string
}

func main() {
	port := flag.Int("port", 47820, "daemon UDP port")
	once := flag.String("once", "", "send this single JSON message and exit")
	stepLen := flag.Duration("step", 8*time.Second, "how long to hold each scripted state")
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

	const song = `"song":"RE-TUNED","artist":"peak divide ft. Rachel Lake","difficulty":"UNBEATABLE","level":20,"highScore":1166540,"accuracy":0.9361,"rank":"S","stars":12.345`
	var started int64
	steps := []step{
		{"idle (menus, stars)", *stepLen, func(int64) string { return `{"v":1,"state":"idle","stars":12.345}` }},
		{"arcade song", *stepLen, func(now int64) string {
			if started == 0 {
				started = now
			}
			return fmt.Sprintf(`{"v":1,"state":"arcade",%s,"startedAt":%d}`, song, started)
		}},
		{"paused", *stepLen, func(int64) string { return fmt.Sprintf(`{"v":1,"state":"arcade",%s,"paused":true}`, song) }},
		{"resumed (timer shifted by the pause)", *stepLen, func(int64) string {
			return fmt.Sprintf(`{"v":1,"state":"arcade",%s,"startedAt":%d}`, song, started+int64(stepLen.Seconds()))
		}},
		{"results (new PB)", *stepLen, func(int64) string {
			return `{"v":1,"state":"results","song":"RE-TUNED","artist":"peak divide ft. Rachel Lake","difficulty":"UNBEATABLE","level":20,"highScore":1201234,"accuracy":0.9512,"rank":"S+","stars":12.401}`
		}},
		{"story mode", *stepLen, func(int64) string { return `{"v":1,"state":"story","chapter":"Chapter 4","stars":12.401}` }},
		{"idle again", *stepLen, func(int64) string { return `{"v":1,"state":"idle","stars":12.401}` }},
		{"closed", 0, func(int64) string { return `{"v":1,"state":"closed"}` }},
	}

	for _, st := range steps {
		log.Printf("-> %s", st.name)
		msg := st.json(time.Now().Unix())
		send(msg)
		deadline := time.Now().Add(st.hold)
		for time.Now().Before(deadline) {
			time.Sleep(2 * time.Second)
			send(msg)
		}
	}
	log.Printf("done")
}
