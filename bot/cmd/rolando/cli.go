package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"rolando/internal/cluster"
	"rolando/internal/config"
	"rolando/internal/guilds"
	"rolando/internal/logger"

	"github.com/disgoorg/snowflake/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const usage = `usage:
  rolando                          run the bot
  rolando migrate split-db [path]  split the legacy single rolando.db into data/guilds/<id>/guild.db
  rolando rebalance                move guild dirs this node no longer owns to REBALANCE_DEST`

func runSubcommand(args []string) {
	var err error
	switch {
	case len(args) >= 2 && args[0] == "migrate" && args[1] == "split-db":
		path := "rolando.db"
		if len(args) > 2 {
			path = args[2]
		}
		err = splitDB(path)
	case args[0] == "rebalance":
		err = rebalance()
	default:
		fmt.Println(usage)
		os.Exit(2)
	}
	if err != nil {
		logger.Fatalf("%v", err)
	}
}

func splitDB(path string) error {
	legacy, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{})
	if err != nil {
		return err
	}
	var configs []guilds.Config
	if err := legacy.Table("chain_configs").Find(&configs).Error; err != nil {
		return err
	}
	for i, cfg := range configs {
		dir := filepath.Join(config.DataDir, "guilds", cfg.ID)
		if _, err := os.Stat(dir); err == nil {
			logger.Infof("[%d/%d] %s exists, skipping", i+1, len(configs), cfg.Name)
			continue
		}
		n, err := splitGuild(legacy, dir, cfg)
		if err != nil {
			os.RemoveAll(dir)
			return fmt.Errorf("%s: %w", cfg.ID, err)
		}
		logger.Infof("[%d/%d] %s: %d messages", i+1, len(configs), cfg.Name, n)
	}
	return nil
}

func splitGuild(legacy *gorm.DB, dir string, cfg guilds.Config) (int, error) {
	g, err := guilds.Open(dir, cfg.ID)
	if err != nil {
		return 0, err
	}
	defer g.Close()
	if err := g.DB.Create(&cfg).Error; err != nil {
		return 0, err
	}
	var last int64
	total := 0
	for {
		var rows []struct {
			ID      int64
			Content string
		}
		err := legacy.Table("messages").Select("id, content").Where("guild_id = ? AND id > ?", cfg.ID, last).
			Order("id").Limit(5000).Scan(&rows).Error
		if err != nil || len(rows) == 0 {
			return total, err
		}
		texts := make([]string, len(rows))
		for i, r := range rows {
			texts[i] = r.Content
		}
		if err := g.AddMessages(texts); err != nil {
			return total, err
		}
		last = rows[len(rows)-1].ID
		total += len(rows)
	}
}

// rebalance moves every guild directory owned by another node under the
// current SHARD_COUNT/NODE_COUNT to REBALANCE_DEST (an rsync destination
// template taking the node index "rolando-%d:/data/guilds/")
func rebalance() error {
	dest := os.Getenv("REBALANCE_DEST")
	root := filepath.Join(config.DataDir, "guilds")
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	moved := 0
	for _, e := range entries {
		id, err := snowflake.Parse(e.Name())
		if err != nil || cluster.IsLocal(id) {
			continue
		}
		if dest == "" {
			return fmt.Errorf("guild %s belongs to node %d: set REBALANCE_DEST (not needed on a shared volume)", id, cluster.OwnerNode(id))
		}
		src := filepath.Join(root, e.Name())
		cmd := exec.Command("rsync", "-a", src, fmt.Sprintf(dest, cluster.OwnerNode(id)))
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("rsync %s: %w", id, err)
		}
		if err := os.RemoveAll(src); err != nil {
			return err
		}
		moved++
	}
	logger.Infof("moved %d guilds", moved)
	return nil
}
