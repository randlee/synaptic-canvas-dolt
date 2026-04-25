package questionnaire

import (
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/repo"
)

// AnswerSet holds collected answers for install-time questions.
type AnswerSet struct {
	Values map[string]any `json:"values"`
}

// CollectDefaults derives deterministic answers from question defaults and repo profile.
func CollectDefaults(questions []models.PackageQuestion, profile repo.Profile) AnswerSet {
	values := make(map[string]any, len(questions))
	for _, question := range questions {
		values[question.QuestionID] = defaultFor(question, profile)
	}
	return AnswerSet{Values: values}
}

func defaultFor(question models.PackageQuestion, profile repo.Profile) any {
	switch question.Type {
	case models.QuestionMulti:
		if strings.HasPrefix(question.DefaultVal, "repo.") {
			switch strings.TrimPrefix(question.DefaultVal, "repo.") {
			case "languages":
				return append([]string(nil), profile.Repo.Languages...)
			case "frameworks":
				return append([]string(nil), profile.Repo.Frameworks...)
			case "test_frameworks":
				return append([]string(nil), profile.Repo.TestFrameworks...)
			}
		}
		if question.DefaultVal == "" {
			return []string{}
		}
		return splitCSV(question.DefaultVal)
	case models.QuestionConfirm:
		return strings.EqualFold(question.DefaultVal, "true")
	case models.QuestionAuto:
		switch strings.TrimPrefix(question.DefaultVal, "repo.") {
		case "name":
			return profile.Repo.Name
		case "primary_language":
			return profile.Repo.PrimaryLanguage
		case "ci_system":
			return profile.Repo.CISystem
		default:
			return question.DefaultVal
		}
	default:
		return question.DefaultVal
	}
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
