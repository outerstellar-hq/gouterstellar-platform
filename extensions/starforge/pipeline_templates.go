package starforge

import "fmt"

const (
	pipelineKindSleepSeries = "sleep-series"
	sleepSeriesSchema       = 1
)

type pipelineTemplate struct {
	Kind          string
	SchemaVersion int
	Page          string
	Build         func(SleepCatalog) sleepSeriesPage
}

type pipelineTemplateRegistry struct {
	templates map[string]pipelineTemplate
}

func newPipelineTemplateRegistry() pipelineTemplateRegistry {
	registry := pipelineTemplateRegistry{templates: map[string]pipelineTemplate{}}
	registry.register(pipelineTemplate{
		Kind:          pipelineKindSleepSeries,
		SchemaVersion: sleepSeriesSchema,
		Page:          "starforge_sleep_series",
		Build:         buildSleepSeriesPage,
	})
	return registry
}

func (r pipelineTemplateRegistry) register(template pipelineTemplate) {
	r.templates[pipelineTemplateKey(template.Kind, template.SchemaVersion)] = template
}

func (r pipelineTemplateRegistry) selectTemplate(kind string, schemaVersion int) (pipelineTemplate, error) {
	template, ok := r.templates[pipelineTemplateKey(kind, schemaVersion)]
	if !ok {
		return pipelineTemplate{}, fmt.Errorf("unsupported Starforge pipeline template %q schema %d", kind, schemaVersion)
	}
	return template, nil
}

func pipelineTemplateKey(kind string, schemaVersion int) string {
	return fmt.Sprintf("%s:%d", kind, schemaVersion)
}
