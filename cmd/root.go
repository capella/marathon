package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uber-go/zap"
)

// RootCmd is the root command for marathon CLI application
var RootCmd = &cobra.Command{
	Use:   "marathon",
	Short: "Marathon sends push-notifications",
	Long:  `Use marathon to send push-notifications to apple (apns) and google (gcm).`,
}

// Execute runs RootCmd to initialize marathon CLI application
func Execute(cmd *cobra.Command, l zap.Logger) {
	if err := cmd.Execute(); err != nil {
		l.Fatal("Error", zap.Error(err))
		os.Exit(-1)
	}
}

func init() {}

// InitConfig reads in config file and ENV variables if set.
func InitConfig(l zap.Logger, configFile string) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		l.Fatal("Config file does not exist", zap.String("configFile", configFile))
		os.Exit(-1)
	}
	viper.SetConfigFile(configFile)

	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("marathon")
	viper.AddConfigPath(".") // optionally look for config in the working directory
	viper.AutomaticEnv()     // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		l.Fatal("Error", zap.Error(err))
		os.Exit(-1)
	}
	l.Info(
		"Using config file",
		zap.String("Config file", viper.ConfigFileUsed()),
	)
}
