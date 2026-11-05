package k8s

import (
	"github.com/jeefy/convergence/internal/models"
	"github.com/jeefy/convergence/internal/templates"
)

// entry is one pinball model as rendered into pinball.yaml and agentgateway.
type entry struct {
	Name         string
	Ref          string // agentgateway provider name serving it
	Upstream     string
	PromptTokens int
}

type templateData struct {
	*ClusterOptions
	R            *models.Resolved
	Entries      []entry
	Fallback     string
	UseOpenAI    bool
	UseAnthropic bool
	Images       templates.Images
}

func newTemplateData(opts *ClusterOptions, res *models.Resolved) *templateData {
	imgs := templates.PinnedImages
	if opts.ImageAgentGateway != "" {
		imgs.AgentGateway = opts.ImageAgentGateway
	}
	if opts.ImageStandIn != "" {
		imgs.StandIn = opts.ImageStandIn
	}
	if opts.ImagePinball != "" {
		imgs.Pinball = opts.ImagePinball
	}
	if opts.ImageCopilotAPI != "" {
		imgs.CopilotAPI = opts.ImageCopilotAPI
	}

	d := &templateData{
		ClusterOptions: opts,
		R:              res,
		Fallback:       res.Fallback,
		UseOpenAI:      res.UsesFamily(models.UpstreamOpenAI),
		UseAnthropic:   res.UsesFamily(models.UpstreamAnthropic),
		Images:         imgs,
	}
	for _, m := range res.Models {
		ref := "copilot-completions"
		switch m.UpstreamProvider {
		case models.UpstreamOpenAI:
			ref = "openai-direct"
		case models.UpstreamAnthropic:
			ref = "anthropic-direct"
		}
		d.Entries = append(d.Entries, entry{m.PinballName, ref, m.UpstreamModel, m.PromptTokens})
	}
	return d
}
