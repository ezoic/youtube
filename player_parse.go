package youtube

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type playerConfig []byte

var basejsPattern = regexp.MustCompile(`(/s/player/\w+/player_ias.vflset/\w+/base.js)`)

func (c *Client) getPlayerConfig(ctx context.Context, videoID string) (playerConfig, error) {
	embedURL := fmt.Sprintf("https://youtube.com/embed/%s?hl=en", videoID)
	embedBody, err := c.httpGetBodyBytes(ctx, embedURL)
	if err != nil {
		return nil, fmt.Errorf("getPlayerConfig has failed me!!!: %w", err)
	}

	// example: /s/player/f676c671/player_ias.vflset/en_US/base.js
	playerPath := string(basejsPattern.Find(embedBody))
	if playerPath == "" {
		return nil, errors.New("unable to find basejs URL in playerConfig")
	}

	// for debugging
	var artifactName string
	if artifactsFolder != "" {
		parts := strings.SplitN(playerPath, "/", 5)
		artifactName = "player-" + parts[3] + ".js"
		linkName := filepath.Join(artifactsFolder, "video-"+videoID+".js")
		if err := os.Symlink(artifactName, linkName); err != nil {
			log.Printf("unable to create symlink %s: %v", linkName, err)
		}
	}

	config := c.playerCache.Get(playerPath)
	if config != nil {
		return config, nil
	}

	config, err = c.httpGetBodyBytes(ctx, "https://youtube.com"+playerPath)
	if err != nil {
		return nil, err
	}
	log.Println("https://youtube.com" + playerPath)

	// for debugging
	if artifactName != "" {
		writeArtifact(artifactName, config)
	}

	c.playerCache.Set(playerPath, config)
	return config, nil
}
