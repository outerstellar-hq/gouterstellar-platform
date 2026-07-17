package starforge

import (
	"net/url"
	"sort"
	"strings"
)

var fixedSleepStages = []string{"text", "voice", "images", "720p", "4k", "subtitles", "upload"}

func normalizeSleepCatalog(payload upstreamSleepCatalog) (SleepCatalog, error) {
	stories := make([]SleepStory, 0, len(payload.Stories))
	for _, story := range payload.Stories {
		if strings.TrimSpace(story.ID) == "" || strings.TrimSpace(story.Title) == "" {
			return SleepCatalog{}, ErrMalformedResponse
		}
		episodes := make([]SleepEpisode, 0, len(story.Episodes))
		for _, episode := range story.Episodes {
			normalized, err := normalizeSleepEpisode(episode)
			if err != nil {
				return SleepCatalog{}, err
			}
			episodes = append(episodes, normalized)
		}
		sort.SliceStable(episodes, func(i, j int) bool {
			return episodes[i].Order < episodes[j].Order
		})
		stories = append(stories, SleepStory{
			ID:       strings.TrimSpace(story.ID),
			Title:    strings.TrimSpace(story.Title),
			Order:    story.Order,
			Episodes: episodes,
		})
	}
	sort.SliceStable(stories, func(i, j int) bool {
		return stories[i].Order < stories[j].Order
	})
	return SleepCatalog{Stories: stories}, nil
}

func normalizeSleepEpisode(episode upstreamSleepEpisode) (SleepEpisode, error) {
	if strings.TrimSpace(episode.ID) == "" || strings.TrimSpace(episode.Title) == "" {
		return SleepEpisode{}, ErrMalformedResponse
	}
	stages := make([]SleepStage, 0, len(fixedSleepStages))
	for _, name := range fixedSleepStages {
		stage := episode.Stages[name]
		stages = append(stages, SleepStage{
			Name:          name,
			Status:        statusOrMissing(stage.Status),
			LastUpdatedAt: stage.LastUpdatedAt,
		})
	}
	artifacts := make([]SleepArtifact, 0, len(episode.Artifacts))
	for _, artifact := range episode.Artifacts {
		if strings.TrimSpace(artifact.Label) == "" || strings.TrimSpace(artifact.State) == "" {
			return SleepEpisode{}, ErrMalformedResponse
		}
		if artifact.URL != "" {
			parsed, err := url.Parse(artifact.URL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
				return SleepEpisode{}, ErrMalformedResponse
			}
		}
		artifacts = append(artifacts, SleepArtifact{
			Label:     strings.TrimSpace(artifact.Label),
			URL:       artifact.URL,
			State:     strings.TrimSpace(artifact.State),
			ExpiresAt: artifact.ExpiresAt,
		})
	}
	return SleepEpisode{
		ID:            strings.TrimSpace(episode.ID),
		Title:         strings.TrimSpace(episode.Title),
		Order:         episode.Order,
		Status:        statusOrMissing(episode.Status),
		LastUpdatedAt: episode.LastUpdatedAt,
		PublicationMetadata: PublicationMetadata{
			Title:       strings.TrimSpace(episode.PublicationMetadata.Title),
			Description: strings.TrimSpace(episode.PublicationMetadata.Description),
		},
		Stages:    stages,
		Artifacts: artifacts,
	}, nil
}

func statusOrMissing(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "missing"
	}
	return status
}
