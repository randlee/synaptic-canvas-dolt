package questionnaire

import (
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
	"github.com/randlee/synaptic-canvas-dolt/pkg/repo"
)

func TestCollectDefaults(t *testing.T) {
	t.Parallel()

	profile := repo.Profile{Repo: repo.RepoSection{
		Name:            "repo",
		PrimaryLanguage: "go",
		Languages:       []string{"go", "python"},
	}}
	answers := CollectDefaults([]models.PackageQuestion{
		{QuestionID: "lang", Type: models.QuestionMulti, DefaultVal: "repo.languages"},
		{QuestionID: "style", Type: models.QuestionChoice, DefaultVal: "direct"},
		{QuestionID: "confirm", Type: models.QuestionConfirm, DefaultVal: "true"},
		{QuestionID: "auto_name", Type: models.QuestionAuto, DefaultVal: "repo.name"},
	}, profile)

	if got := answers.Values["style"]; got != "direct" {
		t.Fatalf("style = %v", got)
	}
	if got := answers.Values["confirm"]; got != true {
		t.Fatalf("confirm = %v", got)
	}
	if got := answers.Values["auto_name"]; got != "repo" {
		t.Fatalf("auto_name = %v", got)
	}
	langs, ok := answers.Values["lang"].([]string)
	if !ok || len(langs) != 2 {
		t.Fatalf("lang = %#v", answers.Values["lang"])
	}
}
