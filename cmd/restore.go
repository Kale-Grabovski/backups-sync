package cmd

import (
	"fmt"
	"os"
	"strings"

	"backupsync/internal/restore"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	restList      bool
	restLatest    bool
	restFile      string
	restTo        string
	restDB        string
	restContainer string
	restDatabase  string
	restUser      string
	restYes       bool
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "List, download and unpack a backup from Backblaze, restore a pg dump",
	Long: `Needs only the backblaze section (id, key, bucket), backups.pwd,
backups.path_7z and, for --db, the db section of the config.

  bsync restore --list
  bsync restore --latest --to /tmp/restore     (or --file NAME)
  bsync restore --db DUMP                      dry run: pg_restore --list
  bsync restore --db DUMP --yes                really restores into the container

Archives are unpacked into an EMPTY directory and never over the live system:
copy the files you need by hand.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		modes := 0
		for _, on := range []bool{restList, restLatest || restFile != "", restDB != ""} {
			if on {
				modes++
			}
		}
		if modes != 1 {
			return fmt.Errorf("choose exactly one of: --list, --latest/--file, --db")
		}

		cfg, err := loadConfig(cfgFile)
		if err != nil {
			return fmt.Errorf("config load failed: %w", err)
		}

		lgCfg := zap.NewProductionConfig()
		lgCfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
		lg, err := lgCfg.Build()
		if err != nil {
			return fmt.Errorf("logger init failed: %w", err)
		}
		defer lg.Sync()

		r, err := restore.New(lg, cfg)
		if err != nil {
			return err
		}

		if restDB != "" {
			return restoreDB(r, cfg.DB.ContainerName, cfg.DB.User, cfg.DB.Database)
		}

		archives, err := r.List()
		if err != nil {
			return err
		}

		if restList {
			if len(archives) == 0 {
				fmt.Println("no archives with this prefix in the bucket")
			}
			for _, a := range archives {
				// made (from the name) first, uploaded second: they differ for an
				// archive the sync re-uploaded after the bucket retention
				fmt.Printf("%s  %s  %10d  %s\n", a.When().Format("2006-01-02 15:04"),
					a.Uploaded.Format("2006-01-02 15:04"), a.Size, a.Name)
			}
			return nil
		}

		a, err := restore.Pick(archives, restFile)
		if err != nil {
			return err
		}
		return restoreArchive(r, a)
	},
}

func restoreArchive(r *restore.Restorer, a restore.Archive) error {
	if entries, err := os.ReadDir(restTo); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s is not empty, pick an empty directory", restTo)
	}
	if err := os.MkdirAll(restTo, 0o700); err != nil {
		return err
	}

	path, err := r.Download(a, restTo)
	if err != nil {
		return err
	}
	if err := r.Extract(path, restTo); err != nil {
		return err
	}
	// the encrypted archive stays next to the files: delete it when done
	fmt.Printf("unpacked %s into %s (archive kept: %s)\n", a.Name, restTo, path)

	dumps, err := restore.FindDumps(restTo)
	if err != nil {
		return err
	}
	if len(dumps) == 0 {
		fmt.Println("no *.dump inside")
		return nil
	}
	fmt.Println("dumps inside, newest first:")
	for _, d := range dumps {
		fmt.Println("  ", d)
	}
	fmt.Printf("next: bsync restore --config %s --db %s            (dry run)\n", cfgFile, dumps[0])
	return nil
}

func restoreDB(r *restore.Restorer, cContainer, cUser, cDatabase string) error {
	container := firstNonEmpty(restContainer, cContainer)
	user := firstNonEmpty(restUser, cUser)
	database := firstNonEmpty(restDatabase, cDatabase)
	if container == "" || user == "" || database == "" {
		return fmt.Errorf("container, user and database are required (db section or flags)")
	}

	out, err := r.PgRestore(restDB, container, user, database, restYes)
	if err != nil {
		return err
	}
	if !restYes {
		lines := strings.Count(out, "\n")
		fmt.Printf("dump is readable, %d lines in its table of contents.\n", lines)
		fmt.Printf("dry run only. To restore into container=%s database=%s add --yes\n", container, database)
		fmt.Println("(this drops and recreates the objects of the dump in that database)")
		return nil
	}
	fmt.Printf("restored into container=%s database=%s\n", container, database)
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func init() {
	f := restoreCmd.Flags()
	f.BoolVar(&restList, "list", false, "list archives in the bucket, newest first")
	f.BoolVar(&restLatest, "latest", false, "download and unpack the newest archive")
	f.StringVar(&restFile, "file", "", "download and unpack this archive (name from --list)")
	f.StringVar(&restTo, "to", "restore", "empty directory to unpack into")
	f.StringVar(&restDB, "db", "", "pg_dump file to restore (custom format)")
	f.StringVar(&restContainer, "container", "", "postgres container (default: db.container_name)")
	f.StringVar(&restDatabase, "database", "", "target database (default: db.database)")
	f.StringVar(&restUser, "user", "", "postgres user (default: db.user)")
	f.BoolVar(&restYes, "yes", false, "really restore the dump, without it only a dry run")
	rootCmd.AddCommand(restoreCmd)
}
