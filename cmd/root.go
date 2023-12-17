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
			fill_color_begin := ""
			fill_color_end := ""
			if tag.IsActive {
				fill_color_begin = "%{B#005577}"
			}
			if tag.IsUrgent {
				fill_color_begin = "%{B#005577}"
			}
			if fill_color_begin != "" {
				fill_color_end = "%{B-}"
			}

			res = append(res, fmt.Sprintf("%s%s %s %s%s", "%{A1:dwm-msg run_command view "+strconv.Itoa(tag.BitMask)+":}", fill_color_begin, tag.Name, fill_color_end, "%{A}"))

		}
	}
	res = append(res, monitor.Layout.Symbol.Current)
	fmt.Println(strings.Join(res, " "))
}

func changeTags(tags []ipc.Tag, state ipc.IPCTagChangeEvent) []ipc.Tag {
	old_active := bitMaskToTagIds(state.Event.OldState.Selected)
	for _, v := range old_active {
		tags[v].IsActive = false
	}

	old_urgent := bitMaskToTagIds(state.Event.OldState.Urgent)
	for _, v := range old_urgent {
		tags[v].IsUrgent = false
	}

	old_occupied := bitMaskToTagIds(state.Event.OldState.Occupied)
	for _, v := range old_occupied {
		tags[v].IsOccupied = false
	}

	new_active := bitMaskToTagIds(state.Event.NewState.Selected)
	for _, v := range new_active {
		tags[v].IsActive = true
	}

	new_urgent := bitMaskToTagIds(state.Event.NewState.Urgent)
	for _, v := range new_urgent {
		tags[v].IsUrgent = true
	}

	new_occupied := bitMaskToTagIds(state.Event.NewState.Occupied)
	for _, v := range new_occupied {
		tags[v].IsOccupied = true
	}

	return tags
}

var (
	mon_number int
	rootCmd    = &cobra.Command{
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
			buf_size, err := c.Read(buf)
			raw_b := buf[ipc.HEADER_LEN : buf_size-1]
			var monitors []ipc.Monitor
			err = json.Unmarshal(raw_b, &monitors)
			var monitor ipc.Monitor
			for _, mon := range monitors {
				if mon.Number == mon_number {
					monitor = mon
				}
			}

			_, err = c.Write(ipc.GenerateMessage(make([]byte, 0), ipc.IPC_TYPE_GET_TAGS))
			if err != nil {
				log.Fatalln(err)
			}

			buf = make([]byte, 1024)
			buf_size, err = c.Read(buf)
			raw_b = buf[ipc.HEADER_LEN : buf_size-1]
			var tags []ipc.Tag
			err = json.Unmarshal(raw_b, &tags)

			sort.Slice(tags, func(i, j int) bool {
				return tags[i].BitMask < tags[j].BitMask
			})

			new_active := bitMaskToTagIds(monitor.TagState.Selected)
			for _, v := range new_active {
				tags[v].IsActive = true
			}

			new_urgent := bitMaskToTagIds(monitor.TagState.Urgent)
			for _, v := range new_urgent {
				tags[v].IsUrgent = true
			}

			new_occupied := bitMaskToTagIds(monitor.TagState.Occupied)
			for _, v := range new_occupied {
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
					payload_size := binary.LittleEndian.Uint32(buf[next+ipc.MAGIC_LEN : next+ipc.MAGIC_LEN+4])
					raw_b := buf[next+ipc.HEADER_LEN : next+ipc.HEADER_LEN+int(payload_size)-1]

					var raw_json map[string]interface{}
					err = json.Unmarshal(raw_b, &raw_json)
					if err != nil {
						log.Fatalf("Unable to marshal JSON due to %s", err)
					}

					for k := range raw_json {
						if k == "tag_change_event" {
							var event ipc.IPCTagChangeEvent
							err = json.Unmarshal(raw_b, &event)
							if event.Event.MonitorNumber == monitor.Number {
								tags = changeTags(tags, event)
								printResult(tags, monitor)
							}
							break
						}

						if k == "layout_change_event" {
							var event ipc.IPCLayoutChangeEvent
							err = json.Unmarshal(raw_b, &event)
							if event.Event.MonitorNumber == monitor.Number {
								monitor.Layout.Symbol.Current = event.Event.NewSymbol
								printResult(tags, monitor)
							}
							break
						}
					}

					next += ipc.HEADER_LEN + int(payload_size)
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
	rootCmd.Flags().IntVar(&mon_number, "monitor", 0, "monitor num we want to process (see dwm-polybar monitors --help)")
}
