package starforge

import (
	"net/http"
	"strings"
)

type sleepSeriesPage struct {
	Stories     []sleepStoryView
	Summary     sleepSeriesSummary
	Unavailable bool
}

type sleepSeriesSummary struct {
	Stories  int
	Episodes int
	Blocked  int
}

type sleepStoryView struct {
	ID       string
	Title    string
	Episodes []sleepEpisodeView
}

type sleepEpisodeView struct {
	ID                     string
	Title                  string
	Status                 string
	StatusClass            string
	LastUpdatedLabel       string
	PublicationTitle       string
	PublicationDescription string
	HasPublicationCopy     bool
	Stages                 []sleepStageView
	Artifacts              []sleepArtifactView
}

type sleepStageView struct {
	Name             string
	Label            string
	Status           string
	StatusClass      string
	LastUpdatedLabel string
}

type sleepArtifactView struct {
	Label      string
	URL        string
	State      string
	StateClass string
	ExpiresAt  string
	HasLink    bool
}

func (e *Extension) sleepSeries(w http.ResponseWriter, r *http.Request) {
	page := sleepSeriesPage{Unavailable: true}
	template, err := e.pipelineTemplates.selectTemplate(pipelineKindSleepSeries, sleepSeriesSchema)
	if err != nil {
		http.Error(w, "Unsupported Starforge pipeline template", http.StatusInternalServerError)
		return
	}
	catalog, err := e.client.SleepCatalog(r.Context())
	if err == nil {
		page = template.Build(catalog)
	}
	if err := e.pages.Render(w, r, template.Page, page); err != nil {
		http.Error(w, "Failed to render Starforge pipeline", http.StatusInternalServerError)
	}
}

func buildSleepSeriesPage(catalog SleepCatalog) sleepSeriesPage {
	page := sleepSeriesPage{Stories: make([]sleepStoryView, 0, len(catalog.Stories))}
	for _, story := range catalog.Stories {
		storyView := sleepStoryView{ID: story.ID, Title: story.Title, Episodes: make([]sleepEpisodeView, 0, len(story.Episodes))}
		page.Summary.Stories++
		for _, episode := range story.Episodes {
			page.Summary.Episodes++
			episodeView := sleepEpisodeView{
				ID:                     episode.ID,
				Title:                  episode.Title,
				Status:                 episode.Status,
				StatusClass:            statusClass(episode.Status),
				LastUpdatedLabel:       formatWorkerTime(episode.LastUpdatedAt),
				PublicationTitle:       episode.PublicationMetadata.Title,
				PublicationDescription: episode.PublicationMetadata.Description,
				HasPublicationCopy:     strings.TrimSpace(episode.PublicationMetadata.Title) != "" || strings.TrimSpace(episode.PublicationMetadata.Description) != "",
				Stages:                 make([]sleepStageView, 0, len(episode.Stages)),
				Artifacts:              make([]sleepArtifactView, 0, len(episode.Artifacts)),
			}
			if statusClass(episode.Status) == "status-blocked" {
				page.Summary.Blocked++
			}
			for _, stage := range episode.Stages {
				episodeView.Stages = append(episodeView.Stages, sleepStageView{
					Name:             stage.Name,
					Label:            stageLabel(stage.Name),
					Status:           stage.Status,
					StatusClass:      statusClass(stage.Status),
					LastUpdatedLabel: formatWorkerTime(stage.LastUpdatedAt),
				})
			}
			for _, artifact := range episode.Artifacts {
				state := artifactState(artifact)
				episodeView.Artifacts = append(episodeView.Artifacts, sleepArtifactView{
					Label:      artifact.Label,
					URL:        artifact.URL,
					State:      state,
					StateClass: artifactStateClass(state),
					ExpiresAt:  formatWorkerTime(artifact.ExpiresAt),
					HasLink:    strings.TrimSpace(artifact.URL) != "" && state != "missing" && state != "expired",
				})
			}
			storyView.Episodes = append(storyView.Episodes, episodeView)
		}
		page.Stories = append(page.Stories, storyView)
	}
	return page
}

func stageLabel(name string) string {
	switch name {
	case "720p":
		return "720p"
	case "4k":
		return "4K"
	default:
		name = strings.ReplaceAll(name, "-", " ")
		if name == "" {
			return "Unknown"
		}
		return strings.ToUpper(name[:1]) + name[1:]
	}
}

func statusClass(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "complete", "online", "ready":
		return "status-online"
	case "running", "pending", "queued":
		return "status-pending"
	case "failed", "blocked", "expired":
		return "status-blocked"
	case "missing":
		return "status-missing"
	default:
		return "status-unknown"
	}
}

func artifactState(artifact SleepArtifact) string {
	state := strings.ToLower(strings.TrimSpace(artifact.State))
	if strings.TrimSpace(artifact.URL) == "" {
		return "missing"
	}
	if state == "" {
		return "available"
	}
	return state
}

func artifactStateClass(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "durable":
		return "artifact-durable"
	case "expired":
		return "artifact-expired"
	case "missing":
		return "artifact-missing"
	default:
		return "artifact-preview"
	}
}
