package core

import (
	"fmt"
	"os"

	"github.com/NetSepio/erebrus/util"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "erebrus",
	Short: "Erebrus is a decentralized VPN node",
	Long: `Erebrus is a decentralized VPN node that provides secure and private internet access.
Complete documentation is available at https://erebrus.io`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Erebrus",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
		fmt.Printf("%s📦 Erebrus Version%s\n", colorGreen, colorReset)
		fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
		fmt.Printf("%s🔖 Version:%s %s\n", colorCyan, colorReset, util.Version)
		fmt.Printf("%s%s%s\n\n", colorYellow, "====================================", colorReset)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of the Erebrus node",
	Run: func(cmd *cobra.Command, args []string) {
		status, err := GetNodeStatus()
		if err != nil {
			fmt.Printf("\n%s%s%s\n", colorRed, err.Error(), colorReset)
			os.Exit(1)
		}

		// Print node status
		fmt.Printf("\n%s%s%s\n", colorYellow, "====================================", colorReset)
		fmt.Printf("%s📊 Node Status%s\n", colorGreen, colorReset)
		fmt.Printf("%s%s%s\n", colorYellow, "====================================", colorReset)
		fmt.Printf("%s🆔 Node ID:%s %s\n", colorCyan, colorReset, status.ID)
		fmt.Printf("%s📛 Name:%s %s\n", colorCyan, colorReset, status.Name)
		fmt.Printf("%s📝 Spec:%s %s\n", colorCyan, colorReset, status.Spec)
		fmt.Printf("%s⚙️  Config:%s %s\n", colorCyan, colorReset, status.Config)
		fmt.Printf("%s🌐 IP Address:%s %s\n", colorCyan, colorReset, status.IPAddress)
		fmt.Printf("%s🗺  Region:%s %s\n", colorCyan, colorReset, status.Region)
		fmt.Printf("%s📍 Location:%s %s\n", colorCyan, colorReset, status.Location)
		fmt.Printf("%s👤 Owner:%s %s\n", colorCyan, colorReset, status.Owner.Hex())
		fmt.Printf("%s🎫 Token ID:%s %v\n", colorCyan, colorReset, status.TokenID)
		fmt.Printf("%s%s Status:%s %s %s\n", colorCyan, status.GetStatusEmoji(), colorReset, status.GetStatusText(), colorReset)

		if status.Checkpoint != "" {
			fmt.Printf("%s📡 Latest Checkpoint:%s %s\n", colorCyan, colorReset, status.Checkpoint)
		}
		
		fmt.Printf("%s%s%s\n\n", colorYellow, "====================================", colorReset)
	},
}

var deactivateCmd = &cobra.Command{
	Use:   "deactivate",
	Short: "Deactivate the Erebrus node",
	Run: func(cmd *cobra.Command, args []string) {
		if err := DeactivateNode(); err != nil {
			fmt.Printf("\n%s❌ Error: %s%s\n", colorRed, err.Error(), colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✅ Node successfully deactivated%s\n", colorGreen, colorReset)
	},
}

var activateCmd = &cobra.Command{
	Use:   "activate",
	Short: "Activate the Erebrus node",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ActivateNode(); err != nil {
			fmt.Printf("\n%s❌ Error: %s%s\n", colorRed, err.Error(), colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✅ Node successfully activated%s\n", colorGreen, colorReset)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(deactivateCmd)
	rootCmd.AddCommand(activateCmd)
}

