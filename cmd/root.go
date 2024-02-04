package cmd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	ipc "github.com/Andrmist/dwm-polybar/utils"
	"github.com/spf13/cobra"
)

func bitMaskToTagIds(bitmask int) []int {
	var bools []int
	for bitmask/2 != 0 {
		bools = append(bools, bitmask%2)
		bitmask /= 2
	}
	bools = append(bools, bitmask%2)
	var res []int
	for i, v := range bools {
		if v == 1 {
			res = append(res, i)
		}
	}
	return res
}

func tagIdsToBitMask(tags []int) int {
	res := 0
	for _, v := range tags {
		res += int(math.Pow(2, float64(v)))
	}
	return res
}

func printResult(tags []ipc.Tag, monitor ipc.Monitor) {
	var res []string
	for _, tag := range tags {
		if tag.IsOccupied || tag.IsActive {
			tagslice := make([]int, 1)
			tagslice[0] = tag.BitMask
			fillColorBegin := ""
			fillColorEnd := ""
			if tag.IsActive {
				fillColorBegin = "%{B#005577}"
			}
			if tag.IsUrgent {
				fillColorBegin = "%{B#005577}"
			}
			if fillColorBegin != "" {
				fillColorEnd = "%{B-}"
			}

			res = append(res, fmt.Sprintf("%s%s %s %s%s", "%{A1:dwm-msg run_command view "+strconv.Itoa(tag.BitMask)+":}", fillColorBegin, tag.Name, fillColorEnd, "%{A}"))

		}
	}
	res = append(res, monitor.Layout.Symbol.Current)
	fmt.Println(strings.Join(res, " "))
}

func changeTags(tags []ipc.Tag, state ipc.IPCTagChangeEvent) []ipc.Tag {
	oldActive := bitMaskToTagIds(state.Event.OldState.Selected)
	for _, v := range oldActive {
		tags[v].IsActive = false
	}

	oldUrgent := bitMaskToTagIds(state.Event.OldState.Urgent)
	for _, v := range oldUrgent {
		tags[v].IsUrgent = false
	}

	oldOccupied := bitMaskToTagIds(state.Event.OldState.Occupied)
	for _, v := range oldOccupied {
		tags[v].IsOccupied = false
	}

	newActive := bitMaskToTagIds(state.Event.NewState.Selected)
	for _, v := range newActive {
		tags[v].IsActive = true
	}

	newUrgent := bitMaskToTagIds(state.Event.NewState.Urgent)
	for _, v := range newUrgent {
		tags[v].IsUrgent = true
	}

	newOccupied := bitMaskToTagIds(state.Event.NewState.Occupied)
	for _, v := range newOccupied {
		tags[v].IsOccupied = true
	}

	return tags
}

var (
	monNumber int
	rootCmd   = &cobra.Command{
		Use:   "dwm-polybar",
		Short: "golang app as a module for polybar to show information about dwm tags and layouts",
		Long: `dwm-polybar - golang app as a module for polybar to show information about dwm tags and layouts
this app will produce new line with information each time you switch tags or layout

config:
add it as a script module for your polybar configuration:

[module/dwm]
type = custom/script
exec = dwm-polybar --monitor 1 # see dwm-polybar monitors --help
tail = true`,
		Run: func(cmd *cobra.Command, args []string) {
			var buf []byte
			c, err := net.Dial("unix", "/tmp/dwm.sock")
			if err != nil {
				log.Println(err)
				return
			}
			defer c.Close()

			// get monitors and get selected
			buf = make([]byte, 10000)
			_, err = c.Write(ipc.GenerateMessage(make([]byte, 0), ipc.IPC_TYPE_GET_MONITORS))
			if err != nil {
				log.Println(err)
				return
			}
			bufSize, err := c.Read(buf)
			rawB := buf[ipc.HEADER_LEN : bufSize-1]
			var monitors []ipc.Monitor
			err = json.Unmarshal(rawB, &monitors)
			var monitor ipc.Monitor
			for _, mon := range monitors {
				if mon.Number == monNumber {
					monitor = mon
				}
			}

			_, err = c.Write(ipc.GenerateMessage(make([]byte, 0), ipc.IPC_TYPE_GET_TAGS))
			if err != nil {
				log.Fatalln(err)
			}

			buf = make([]byte, 1024)
			bufSize, err = c.Read(buf)
			rawB = buf[ipc.HEADER_LEN : bufSize-1]
			var tags []ipc.Tag
			err = json.Unmarshal(rawB, &tags)

			sort.Slice(tags, func(i, j int) bool {
				return tags[i].BitMask < tags[j].BitMask
			})

			newActive := bitMaskToTagIds(monitor.TagState.Selected)
			for _, v := range newActive {
				tags[v].IsActive = true
			}

			newUrgent := bitMaskToTagIds(monitor.TagState.Urgent)
			for _, v := range newUrgent {
				tags[v].IsUrgent = true
			}

			newOccupied := bitMaskToTagIds(monitor.TagState.Occupied)
			for _, v := range newOccupied {
				tags[v].IsOccupied = true
			}

			printResult(tags, monitor)

			// subscribe to tag and layout updates
			err = ipc.InitSubscribe(&c)
			if err != nil {
				log.Fatalln(err)
			}

			// subscribe loop
			for {
				buf := make([]byte, 1024)
				_, err := c.Read(buf)

				if err != nil {
					if err.Error() == "EOF" {
						for {
							c, err = net.Dial("unix", "/tmp/dwm.sock")
							if err == nil {
								err = ipc.InitSubscribe(&c)
								if err != nil {
									continue
								}
								for i := range tags {
									tags[i].IsActive = false
									tags[i].IsUrgent = false
								}
								tags[0].IsActive = true
								printResult(tags, monitor)
								break
							}
							time.Sleep(time.Duration(500) * time.Millisecond)
							log.Println(err)
						}
					} else {
						log.Println(err)
					}
					continue
				}

				next := 0
				for i := 0; i < strings.Count(string(buf), "DWM-IPC"); i++ {
					payloadSize := binary.LittleEndian.Uint32(buf[next+ipc.MAGIC_LEN : next+ipc.MAGIC_LEN+4])
					rawB := buf[next+ipc.HEADER_LEN : next+ipc.HEADER_LEN+int(payloadSize)-1]

					var rawJson map[string]interface{}
					err = json.Unmarshal(rawB, &rawJson)
					if err != nil {
						log.Fatalf("Unable to marshal JSON due to %s", err)
					}

					for k := range rawJson {
						if k == "tag_change_event" {
							var event ipc.IPCTagChangeEvent
							err = json.Unmarshal(rawB, &event)
							if event.Event.MonitorNumber == monitor.Number {
								tags = changeTags(tags, event)
								printResult(tags, monitor)
							}
							break
						}

						if k == "layout_change_event" {
							var event ipc.IPCLayoutChangeEvent
							err = json.Unmarshal(rawB, &event)
							if event.Event.MonitorNumber == monitor.Number {
								monitor.Layout.Symbol.Current = event.Event.NewSymbol
								printResult(tags, monitor)
							}
							break
						}
					}

					next += ipc.HEADER_LEN + int(payloadSize)
				}
			}

		},
	}
)

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().IntVar(&monNumber, "monitor", 0, "monitor num we want to process (see dwm-polybar monitors --help)")
}
