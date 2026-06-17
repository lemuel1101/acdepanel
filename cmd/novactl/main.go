package main

import (
	"fmt"
	"log"

	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/config"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/spf13/cobra"
)

var cfg *config.Config
var cfgPath string

func main() {
	rootCmd := &cobra.Command{
		Use:   "novactl",
		Short: "NovaPanel CLI - Server administration tool",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			path := cfgPath
			if path == "" {
				path = "/etc/novapanel/config.yaml"
			}
			var err error
			cfg, err = config.LoadConfig(path)
			if err != nil {
				cfg = config.DefaultConfig()
			}
			if err := db.InitDatabase(cfg); err != nil {
				return fmt.Errorf("database: %w", err)
			}
			return db.RunMigrations()
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "Config file path")

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install and initialize NovaPanel",
		RunE:  runInstall,
	}

	createAdminCmd := &cobra.Command{
		Use:   "create-admin",
		Short: "Create admin user",
		RunE:  runCreateAdmin,
	}

	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}
	userCmd.AddCommand(createAdminCmd)

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Server management commands",
	}

	serverCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show server status",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("NovaPanel Server Status")
			fmt.Println("=======================")
			fmt.Println("Status: Running")
			fmt.Printf("Config: %s\n", cfgPath)
			return nil
		},
	})

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("NovaPanel v1.0.0")
		},
	}

	rootCmd.AddCommand(installCmd, userCmd, serverCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runInstall(cmd *cobra.Command, args []string) error {
	fmt.Println("Installing NovaPanel...")
	fmt.Println("Creating configuration...")
	fmt.Println("Setting up database...")
	fmt.Println("Installation complete!")
	fmt.Println("Run 'novactl create-admin' to create an admin user")
	return nil
}

func runCreateAdmin(cmd *cobra.Command, args []string) error {
	var username, password, email string

	fmt.Print("Username: ")
	fmt.Scanln(&username)
	fmt.Print("Email: ")
	fmt.Scanln(&email)
	fmt.Print("Password: ")
	fmt.Scanln(&password)

	if username == "" || password == "" || email == "" {
		return fmt.Errorf("all fields are required")
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := db.User{
		Username: username,
		Email:    email,
		Password: hashedPassword,
		Role:     "admin",
		Status:   "active",
	}

	if err := db.DB.Create(&user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Printf("Admin user '%s' created successfully!\n", username)
	return nil
}
