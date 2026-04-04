package monitor

import (
	"fmt"
	"log"
	"time"

	"github.com/lidarr-utils/internal/common"
	"github.com/lidarr-utils/internal/lidarr"
)

type QueueConfig struct {
	MaxInQueue   int
	DelaySeconds int
	MaxWaitTime  time.Duration
}

type commandInfo struct {
	Name   string
	Status string
}

type SearchQueue struct {
	client *lidarr.Client
	config QueueConfig
	dryRun bool
}

func NewSearchQueue(client *lidarr.Client, config QueueConfig, dryRun bool) *SearchQueue {
	return &SearchQueue{
		client: client,
		config: config,
		dryRun: dryRun,
	}
}

func (q *SearchQueue) ProcessAlbums(albumChan <-chan common.Album) int {
	searched := 0

	for album := range albumChan {
		if q.dryRun {
			log.Printf("  [DRY RUN] Would monitor and search: %s", album.Title)
			searched++
			continue
		}

		if err := q.client.MonitorAlbum(album.ID); err != nil {
			log.Printf("  ERROR: Failed to monitor album %s, skipping search: %v", album.Title, err)
			continue
		}

		if err := q.client.SearchAlbum([]int{album.ID}); err != nil {
			log.Printf("  ERROR: Failed to search for %s: %v", album.Title, err)
			continue
		}

		log.Printf("  Monitored and queued search: %s", album.Title)
		searched++

		if err := q.waitForSlot(); err != nil {
			log.Printf("  ERROR: Failed to check queue, pausing before next search: %v", err)
			time.Sleep(time.Duration(q.config.DelaySeconds) * time.Second)
		}
	}

	return searched
}

func (q *SearchQueue) waitForSlot() error {
	maxWait := q.config.MaxWaitTime
	if maxWait == 0 {
		maxWait = 10 * time.Minute
	}
	deadline := time.Now().Add(maxWait)
	waiting := false

	for {
		commands, err := q.client.GetCommands()
		if err != nil {
			return fmt.Errorf("failed to get commands: %w", err)
		}

		var infos []commandInfo
		for _, cmd := range commands {
			infos = append(infos, commandInfo{Name: cmd.Name, Status: cmd.Status})
		}

		active := countActiveSearches(infos)
		if active < q.config.MaxInQueue {
			if waiting {
				log.Printf("  Search slot available (%d/%d active searches)", active, q.config.MaxInQueue)
			}
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %v waiting for search slot (%d/%d active searches)", maxWait, active, q.config.MaxInQueue)
		}

		if !waiting {
			log.Printf("  Waiting for search slot (%d/%d active searches)...", active, q.config.MaxInQueue)
			waiting = true
		}
		time.Sleep(time.Duration(q.config.DelaySeconds) * time.Second)
	}
}

func countActiveSearches(commands []commandInfo) int {
	count := 0
	for _, cmd := range commands {
		if cmd.Name == "AlbumSearch" && (cmd.Status == "started" || cmd.Status == "queued") {
			count++
		}
	}
	return count
}
