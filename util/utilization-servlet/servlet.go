package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"time"
)

const UpdateInterval = 1 // seconds between measurement probes, default: 3

var currentUtil = 0.0		// current CPU utilization

func getCPUSample() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					fmt.Println("Error: ", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}

func startUtilUpdater() {
	var idle0, total0 uint64
	idle1, total1 := getCPUSample()
	for true {
		time.Sleep(UpdateInterval * time.Second)
		idle0 = idle1	// shift new -> old values
		total0 = total1
		idle1, total1 = getCPUSample()
		idleTicks := float64(idle1 - idle0)
		totalTicks := float64(total1 - total0)
		currentUtil = (totalTicks - idleTicks) / totalTicks
	}
}

func main() {
	go startUtilUpdater() // updates the util variable every defined interval

	l, err := net.Listen("tcp4", ":22442")	// start a tcp server
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go handleConnection(c)
	}
}

func handleConnection(c net.Conn) {
	for true {
		_, err := bufio.NewReader(c).ReadString('\n')	// waits to receive a line
		if err != nil {
			fmt.Println(err)
			break
		}

		_, err = c.Write([]byte(strconv.FormatFloat(currentUtil, 'f', -1, 64) + "\n")) // string to ASCII, write util to socket
		if err != nil {
			fmt.Println(err)
			break
		}
	}
	err := c.Close()
	if err != nil {
		fmt.Println(err)
	}
}
