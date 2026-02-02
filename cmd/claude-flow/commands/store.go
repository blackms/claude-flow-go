// Package commands provides CLI command implementations.
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	appTransfer "github.com/anthropics/claude-flow-go/internal/application/transfer"
	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
)

// Store command flags
var (
	// List flags
	storeListRegistry string
	storeListCategory string
	storeListFeatured bool
	storeListTrending bool
	storeListNewest   bool
	storeListLimit    int
	storeListFormat   string

	// Search flags
	storeSearchQuery     string
	storeSearchCategory  string
	storeSearchLanguage  string
	storeSearchFramework string
	storeSearchTags      string
	storeSearchMinRating float64
	storeSearchVerified  bool
	storeSearchLimit     int

	// Download flags
	storeDownloadName   string
	storeDownloadOutput string
	storeDownloadVerify bool
	storeDownloadImport bool

	// Publish flags
	storePublishInput       string
	storePublishName        string
	storePublishDescription string
	storePublishCategories  string
	storePublishTags        string
	storePublishLicense     string
	storePublishAnonymize   string
	storePublishLanguage    string
	storePublishFramework   string

	// Info flags
	storeInfoName string
	storeInfoJSON bool
)

// StoreCmd is the parent command for pattern store operations.
var StoreCmd = &cobra.Command{
	Use:     "store",
	Aliases: []string{"transfer-store", "ts"},
	Short:   "Pattern store for sharing and distribution",
	Long: `Commands for managing the pattern store.

The pattern store provides:
  - Browse and search patterns from registries
  - Download patterns with verification
  - Publish patterns to IPFS or GCS
  - Pattern information and statistics`,
}

// storeListCmd lists patterns from the registry
var storeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List patterns from the registry",
	Long:    `List available patterns from the pattern registry with optional filtering.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config := transfer.DefaultStoreConfig()
		if storeListRegistry != "" {
			config.Registry = storeListRegistry
		}

		service, err := appTransfer.NewService(config)
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		options := transfer.ListOptions{
			Registry: storeListRegistry,
			Category: storeListCategory,
			Featured: storeListFeatured,
			Trending: storeListTrending,
			Newest:   storeListNewest,
			Limit:    storeListLimit,
			Format:   storeListFormat,
		}

		patterns, err := service.List(options)
		if err != nil {
			return err
		}

		if len(patterns) == 0 {
			fmt.Println("No patterns found")
			return nil
		}

		if storeListFormat == "json" {
			output, _ := json.MarshalIndent(patterns, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		// Table format
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVERSION\tDOWNLOADS\tRATING\tSTATUS")
		fmt.Fprintln(w, strings.Repeat("-", 70))

		for _, p := range patterns {
			status := ""
			if p.Verified {
				status = "verified"
			}
			if p.Featured {
				if status != "" {
					status += ", "
				}
				status += "featured"
			}
			if p.Trending {
				if status != "" {
					status += ", "
				}
				status += "trending"
			}

			fmt.Fprintf(w, "%s\t%s\t%d\t%.1f\t%s\n",
				p.DisplayName, p.Version, p.Downloads, p.Rating, status)
		}
		w.Flush()

		fmt.Printf("\nShowing %d patterns from %s\n", len(patterns), config.Registry)

		return nil
	},
}

// storeSearchCmd searches patterns in the registry
var storeSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search patterns in the registry",
	Long:  `Search for patterns by keyword with optional filters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if storeSearchQuery == "" {
			return fmt.Errorf("search query is required (--query)")
		}

		service, err := appTransfer.NewService(transfer.DefaultStoreConfig())
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		var tags []string
		if storeSearchTags != "" {
			tags = strings.Split(storeSearchTags, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
		}

		options := transfer.SearchOptions{
			Query:     storeSearchQuery,
			Category:  storeSearchCategory,
			Language:  storeSearchLanguage,
			Framework: storeSearchFramework,
			Tags:      tags,
			MinRating: storeSearchMinRating,
			Verified:  storeSearchVerified,
			Limit:     storeSearchLimit,
		}

		patterns, err := service.Search(options)
		if err != nil {
			return err
		}

		if len(patterns) == 0 {
			fmt.Printf("No patterns found for '%s'\n", storeSearchQuery)
			return nil
		}

		// Table format
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tDOWNLOADS\tRATING")
		fmt.Fprintln(w, strings.Repeat("-", 80))

		for _, p := range patterns {
			desc := p.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%.1f\n",
				p.DisplayName, desc, p.Downloads, p.Rating)
		}
		w.Flush()

		fmt.Printf("\nFound %d patterns matching '%s'\n", len(patterns), storeSearchQuery)

		return nil
	},
}

// storeDownloadCmd downloads a pattern
var storeDownloadCmd = &cobra.Command{
	Use:     "download",
	Aliases: []string{"get", "install"},
	Short:   "Download a pattern from the registry",
	Long:    `Download a pattern by name or ID with optional verification.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if storeDownloadName == "" {
			return fmt.Errorf("pattern name is required (--name)")
		}

		service, err := appTransfer.NewService(transfer.DefaultStoreConfig())
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		options := transfer.DownloadOptions{
			Name:       storeDownloadName,
			OutputPath: storeDownloadOutput,
			Verify:     storeDownloadVerify,
			Import:     storeDownloadImport,
		}

		fmt.Printf("Downloading pattern: %s\n", storeDownloadName)

		result, err := service.Download(options)
		if err != nil {
			return err
		}

		fmt.Println("\nDownload Complete!")
		fmt.Printf("  CID:      %s\n", result.CID)
		fmt.Printf("  Path:     %s\n", result.Path)
		fmt.Printf("  Size:     %d bytes\n", result.Size)
		fmt.Printf("  Checksum: %s\n", result.Checksum)
		if result.FromCache {
			fmt.Println("  Source:   cache")
		}
		if result.Verified {
			fmt.Println("  Verified: yes")
		}

		return nil
	},
}

// storePublishCmd publishes a pattern
var storePublishCmd = &cobra.Command{
	Use:     "publish",
	Aliases: []string{"contribute"},
	Short:   "Publish a pattern to the registry",
	Long:    `Publish a pattern file (CFP format) to the pattern store.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if storePublishInput == "" {
			return fmt.Errorf("input file is required (--input)")
		}
		if storePublishName == "" {
			return fmt.Errorf("pattern name is required (--name)")
		}
		if storePublishDescription == "" {
			return fmt.Errorf("description is required (--description)")
		}
		if storePublishCategories == "" {
			return fmt.Errorf("categories are required (--categories)")
		}
		if storePublishTags == "" {
			return fmt.Errorf("tags are required (--tags)")
		}

		service, err := appTransfer.NewService(transfer.DefaultStoreConfig())
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		categories := strings.Split(storePublishCategories, ",")
		for i := range categories {
			categories[i] = strings.TrimSpace(categories[i])
		}

		tags := strings.Split(storePublishTags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}

		anonymize := transfer.AnonymizationStrict
		switch storePublishAnonymize {
		case "minimal":
			anonymize = transfer.AnonymizationMinimal
		case "standard":
			anonymize = transfer.AnonymizationStandard
		case "paranoid":
			anonymize = transfer.AnonymizationParanoid
		}

		options := transfer.PublishOptions{
			InputPath:   storePublishInput,
			Name:        storePublishName,
			Description: storePublishDescription,
			Categories:  categories,
			Tags:        tags,
			License:     storePublishLicense,
			Anonymize:   anonymize,
			Language:    storePublishLanguage,
			Framework:   storePublishFramework,
			Sign:        true,
		}

		fmt.Printf("Publishing pattern: %s\n", storePublishName)

		result, err := service.Publish(options)
		if err != nil {
			return err
		}

		fmt.Println("\nPublish Complete!")
		fmt.Printf("  Pattern ID: %s\n", result.PatternID)
		fmt.Printf("  CID:        %s\n", result.CID)
		fmt.Printf("  Gateway:    %s\n", result.GatewayURL)
		fmt.Printf("  Checksum:   %s\n", result.Checksum)
		fmt.Printf("  Size:       %d bytes\n", result.Size)

		return nil
	},
}

// storeInfoCmd shows pattern information
var storeInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show detailed pattern information",
	Long:  `Display detailed information about a pattern.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if storeInfoName == "" {
			return fmt.Errorf("pattern name is required (--name)")
		}

		service, err := appTransfer.NewService(transfer.DefaultStoreConfig())
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		options := transfer.InfoOptions{
			Name:   storeInfoName,
			AsJSON: storeInfoJSON,
		}

		info, err := service.Info(options)
		if err != nil {
			return err
		}

		if storeInfoJSON {
			output, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		fmt.Println(appTransfer.FormatPatternInfo(info))

		return nil
	},
}

func init() {
	// List command flags
	storeListCmd.Flags().StringVarP(&storeListRegistry, "registry", "r", "claude-flow-official", "Registry name")
	storeListCmd.Flags().StringVarP(&storeListCategory, "category", "c", "", "Filter by category")
	storeListCmd.Flags().BoolVarP(&storeListFeatured, "featured", "f", false, "Show featured patterns")
	storeListCmd.Flags().BoolVarP(&storeListTrending, "trending", "t", false, "Show trending patterns")
	storeListCmd.Flags().BoolVarP(&storeListNewest, "newest", "n", false, "Show newest patterns")
	storeListCmd.Flags().IntVarP(&storeListLimit, "limit", "l", 20, "Maximum results")
	storeListCmd.Flags().StringVar(&storeListFormat, "format", "table", "Output format (table|json)")

	// Search command flags
	storeSearchCmd.Flags().StringVarP(&storeSearchQuery, "query", "q", "", "Search query (required)")
	storeSearchCmd.Flags().StringVarP(&storeSearchCategory, "category", "c", "", "Filter by category")
	storeSearchCmd.Flags().StringVar(&storeSearchLanguage, "language", "", "Filter by language")
	storeSearchCmd.Flags().StringVar(&storeSearchFramework, "framework", "", "Filter by framework")
	storeSearchCmd.Flags().StringVarP(&storeSearchTags, "tags", "t", "", "Filter by tags (comma-separated)")
	storeSearchCmd.Flags().Float64Var(&storeSearchMinRating, "min-rating", 0, "Minimum rating (0-5)")
	storeSearchCmd.Flags().BoolVarP(&storeSearchVerified, "verified", "v", false, "Only verified patterns")
	storeSearchCmd.Flags().IntVarP(&storeSearchLimit, "limit", "l", 20, "Maximum results")

	// Download command flags
	storeDownloadCmd.Flags().StringVarP(&storeDownloadName, "name", "n", "", "Pattern name or ID (required)")
	storeDownloadCmd.Flags().StringVarP(&storeDownloadOutput, "output", "o", "", "Output file path")
	storeDownloadCmd.Flags().BoolVar(&storeDownloadVerify, "verify", true, "Verify checksum")
	storeDownloadCmd.Flags().BoolVar(&storeDownloadImport, "import", false, "Import after download")

	// Publish command flags
	storePublishCmd.Flags().StringVarP(&storePublishInput, "input", "i", "", "Input CFP file (required)")
	storePublishCmd.Flags().StringVarP(&storePublishName, "name", "n", "", "Pattern name (required)")
	storePublishCmd.Flags().StringVarP(&storePublishDescription, "description", "d", "", "Description (required)")
	storePublishCmd.Flags().StringVarP(&storePublishCategories, "categories", "c", "", "Categories (comma-separated, required)")
	storePublishCmd.Flags().StringVarP(&storePublishTags, "tags", "t", "", "Tags (comma-separated, required)")
	storePublishCmd.Flags().StringVarP(&storePublishLicense, "license", "l", "MIT", "SPDX license")
	storePublishCmd.Flags().StringVar(&storePublishAnonymize, "anonymize", "strict", "Anonymization level (minimal|standard|strict|paranoid)")
	storePublishCmd.Flags().StringVar(&storePublishLanguage, "language", "", "Primary language")
	storePublishCmd.Flags().StringVar(&storePublishFramework, "framework", "", "Primary framework")

	// Info command flags
	storeInfoCmd.Flags().StringVarP(&storeInfoName, "name", "n", "", "Pattern name or ID (required)")
	storeInfoCmd.Flags().BoolVar(&storeInfoJSON, "json", false, "Output as JSON")

	// Add subcommands to store
	StoreCmd.AddCommand(storeListCmd)
	StoreCmd.AddCommand(storeSearchCmd)
	StoreCmd.AddCommand(storeDownloadCmd)
	StoreCmd.AddCommand(storePublishCmd)
	StoreCmd.AddCommand(storeInfoCmd)
}
