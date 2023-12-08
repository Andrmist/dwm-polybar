package cmd

import (
	"encoding/json"
	"fmt"
	"net"

	ipc "github.com/Andrmist/dwm-polybar/utils"
	"github.com/spf13/cobra"
)

var monitorsCmd = &cobra.Command{
	Use:   "monitors",
	Short: "helper command to get information about monitors known by dwm",
	Long:  `since we can't predict what monitor you're trying to use, you can see what applications are present on each monitor, so you can see what "--monitor" value you should use`,
	Run: func(cmd *cobra.Command, args []string) {
		var buf []byte
		c, err := net.Dial("unix", "/tmp/dwm.sock")
		if err != nil {
			fmt.Println(err)
			return
		}
		defer c.Close()
		buf = make([]byte, 10000)
		_, err = c.Write(ipc.GenerateMessage(make([]byte, 0), ipc.IPC_TYPE_GET_MONITORS))
		if err != nil {
			fmt.Println(err)
			return
		}
		buf_size, err := c.Read(buf)
		raw_b := buf[ipc.HEADER_LEN : buf_size-1]
		var monitors []ipc.Monitor
		err = json.Unmarshal(raw_b, &monitors)
		for _, mon := range monitors {
			buf = make([]byte, 10000)
			err = ipc.SendStruct(&c, ipc.IPCGetDWMClientPayload{Id: mon.Clients.Selected}, ipc.IPC_TYPE_GET_DWM_CLIENT)
			if err != nil {
				fmt.Println(err)
				return
			}
			buf_size, err := c.Read(buf)
			if err != nil {
				fmt.Println(err)
				return
			}
			raw_b := buf[ipc.HEADER_LEN : buf_size-1]
			var client ipc.Client
			err = json.Unmarshal(raw_b, &client)
			if err != nil {
				fmt.Println(err)
				return
			}

			selected := ""
			if mon.IsSelected {
				selected = " (current)"
			}

			fmt.Printf(`Monitor %d%s:
Selected application name: %s

`, mon.Number, selected, client.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(monitorsCmd)
}
