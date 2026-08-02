package github

import (
	"context"
	"time"

	"github.com/dreuse/prdash/internal/model"
)

const (
	MockViewer = "dreuse"
	mockRepo   = "vobys/folha-spring-boot"
	mockRepo2  = "vobys/other-service"
)

type Mock struct {
	Err error
}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Approve(context.Context, model.PullRequest) error         { return nil }
func (m *Mock) Comment(context.Context, model.PullRequest, string) error { return nil }
func (m *Mock) Merge(context.Context, model.PullRequest, string) error   { return nil }
func (m *Mock) Close(context.Context, model.PullRequest) error           { return nil }
func (m *Mock) Rerun(context.Context, model.PullRequest) error           { return nil }
func (m *Mock) RerunRun(context.Context, model.WorkflowRun) error        { return nil }
func (m *Mock) UpdateBranch(context.Context, model.PullRequest) error    { return nil }

func (m *Mock) RunLog(_ context.Context, run model.WorkflowRun) ([]string, error) {
	if run.InProgress() {
		return nil, ErrLogsNotReady
	}
	job := run.FailingJob
	if job == "" {
		job = "build"
	}
	step := run.FailingStep
	if step == "" {
		step = "Run tests"
	}

	lines := make([]string, 0, 64)
	for i := 0; i < 48; i++ {
		lines = append(lines, job+"\t"+step+"\t"+mockLogNoise[i%len(mockLogNoise)])
	}
	if !run.Failed() {
		return append(lines, job+"\t"+step+"\tBUILD SUCCESS"), nil
	}
	return append(lines,
		job+"\t"+step+"\tERROR Migration V42__rubrica_empresa.sql failed",
		job+"\t"+step+"\tERROR SQL State  : 42S01",
		job+"\t"+step+"\tERROR Message    : Table 'rubrica_empresa' already exists",
		job+"\t"+step+"\tProcess completed with exit code 1",
	), nil
}

var mockLogNoise = []string{
	"Downloading from central: org/springframework/spring-core/6.1.4/spring-core.jar",
	"Compiling 312 source files with javac",
	"Tests run: 48, Failures: 0, Errors: 0, Skipped: 2",
	"Flyway Community Edition 10.4.1 by Redgate",
	"Database: jdbc:postgresql://localhost:5432/folha (PostgreSQL 16.1)",
	"Successfully validated 41 migrations",
}

func day(n int) time.Duration { return time.Duration(n) * 24 * time.Hour }

func checks(passed, failed, running, neutral int) []model.Check {
	out := make([]model.Check, 0, passed+failed+running+neutral)
	names := []string{"build", "unit-tests", "lint", "integration-tests", "sonar", "flyway:migrate",
		"contract-tests", "e2e", "docker", "helm-lint", "owasp", "javadoc", "coverage", "spotless", "license"}
	next := func(i int) string {
		if i < len(names) {
			return names[i]
		}
		return "check"
	}
	i := 0
	for ; i < failed; i++ {
		out = append(out, model.Check{Name: next(i), State: model.CheckFailed})
	}
	for j := 0; j < running; j++ {
		out = append(out, model.Check{Name: next(i), State: model.CheckRunning,
			StartedAt: time.Now().Add(-4 * time.Minute)})
		i++
	}
	for j := 0; j < passed; j++ {
		out = append(out, model.Check{Name: next(i), State: model.CheckPassed})
		i++
	}
	for j := 0; j < neutral; j++ {
		out = append(out, model.Check{Name: next(i), State: model.CheckNeutral})
		i++
	}
	return out
}

func (m *Mock) Fetch(context.Context) (Snapshot, error) {
	if m.Err != nil {
		return Snapshot{}, m.Err
	}
	now := time.Now()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	prs := []model.PullRequest{
		{
			Repo: mockRepo, Number: 12009, Title: "Adição do campo Categoria",
			Author: "LevisFreitas1771", CreatedAt: ago(day(8)), UpdatedAt: ago(3 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feat/categoria",
			BehindBy: 19, Approvals: 1, Additions: 88, Deletions: 12, Changed: 5,
			Reviews: []model.Review{{Login: "alfoltran", State: model.ReviewApproved, SubmittedAt: ago(2 * time.Hour)}},
			Checks:  checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 12012, Title: "SQL idempotente",
			Author: MockViewer, CreatedAt: ago(day(6)), UpdatedAt: ago(30 * time.Minute),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feature/sql-idempotente",
			BehindBy: 19, Additions: 412, Deletions: 86, Changed: 14,
			RequestedReviewers: []string{MockViewer, "alfoltran", "fabiomiziara"},
			Assignees:          []string{MockViewer},
			Checks:             checks(11, 0, 0, 4),
			Labels:             []string{"database"},
		},
		{
			Repo: mockRepo, Number: 11959, Title: "Importador de Árvore Salarial",
			Author: "alfoltran", CreatedAt: ago(day(15)), UpdatedAt: ago(day(2)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feat/importador-arvore",
			BehindBy: 38, Additions: 640, Deletions: 40, Changed: 22,
			RequestedReviewers: []string{MockViewer},
			Checks:             checks(9, 0, 0, 4),
		},
		{
			Repo: mockRepo, Number: 12038, Title: "IntegraGO IPASGO — inventário",
			Author: "lucassousaalves", CreatedAt: ago(day(2)), UpdatedAt: ago(5 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "release/rubrica/esquema-empresa",
			BehindBy: 8, ChangesRequested: 1, Additions: 120, Deletions: 8, Changed: 6,
			Assignees: []string{"theorocha"}, Labels: []string{"integration", "bug"},
			Reviews: []model.Review{{Login: "theorocha", State: model.ReviewChangesRequested, SubmittedAt: ago(6 * time.Hour)}},
			Checks:  checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 12022, Title: "Exclusão de pessoas vinculadas",
			Author: "AlexandreLages", CreatedAt: ago(day(4)), UpdatedAt: ago(day(1)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feature/concurso-lista-classificados",
			BehindBy: 7, ChangesRequested: 1, Additions: 55, Deletions: 210, Changed: 9,
			Reviews: []model.Review{{Login: "alfoltran", State: model.ReviewChangesRequested, SubmittedAt: ago(day(2))}},
			Checks:  checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 11925, Title: "Progressão por desempenho",
			Author: "theorocha", CreatedAt: ago(day(18)), UpdatedAt: ago(day(3)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feat/progressao-desempenho-exercicio",
			BehindBy: 3, ChangesRequested: 1, Additions: 300, Deletions: 20, Changed: 11,
			Checks: checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 12023, Title: "Revert Estagio Probatório",
			Author: "HyakoV3", CreatedAt: ago(day(4)), UpdatedAt: ago(2 * time.Hour),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "revert/estagio-probatorio",
			BehindBy: 7, Additions: 12, Deletions: 400, Changed: 7,
			Checks: checks(13, 2, 0, 0),
		},
		{
			Repo: mockRepo, Number: 11761, Title: "Rubricas por Empresa",
			Author: "alfoltran", CreatedAt: ago(day(50)), UpdatedAt: ago(day(4)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "feat/rubricas-empresa",
			Additions: 210, Deletions: 30, Changed: 12,
			Checks: checks(14, 1, 0, 0),
		},
		{
			Repo: mockRepo, Number: 11731, Title: "chore: Adiciona campo situação",
			Author: "TRios04", CreatedAt: ago(day(54)), UpdatedAt: ago(day(6)),
			Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "chore/campo-situacao",
			BehindBy: 178, Additions: 40, Deletions: 4, Changed: 3,
			Checks: checks(14, 1, 0, 0),
		},
		{
			Repo: mockRepo, Number: 10289, Title: "POC do TJPA",
			Author: "alfoltran", CreatedAt: ago(day(267)), UpdatedAt: ago(day(190)),
			Mergeable: model.MergeableConflict, BaseRef: "master", HeadRef: "poc/tjpa",
			BehindBy: 720, Additions: 900, Deletions: 300, Changed: 60,
			Checks: checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo, Number: 9925, Title: "Release 2025/09-1",
			Author: "alfoltran", CreatedAt: ago(day(320)), UpdatedAt: ago(day(240)),
			Mergeable: model.MergeableConflict, BaseRef: "master", HeadRef: "release/2025-09-1",
			Additions: 30, Deletions: 10, Changed: 4,
			Checks: checks(14, 1, 0, 0),
		},
		{
			Repo: mockRepo, Number: 11952, Title: "WIP: frontend",
			Author: MockViewer, CreatedAt: ago(day(15)), UpdatedAt: ago(day(9)),
			IsDraft: true, Mergeable: model.MergeableYes, BaseRef: "master", HeadRef: "wip/frontend",
			BehindBy: 69, Additions: 1200, Deletions: 40, Changed: 80,
			Checks: checks(11, 4, 0, 0),
		},
		{
			Repo: mockRepo2, Number: 11758, Title: "Autenticação do dispositivo",
			Author: "victoralcan", CreatedAt: ago(day(51)), UpdatedAt: ago(day(20)),
			IsDraft: true, Mergeable: model.MergeableYes, BaseRef: "main", HeadRef: "feat/auth-dispositivo",
			Additions: 220, Deletions: 15, Changed: 14,
			Checks: checks(15, 0, 0, 0),
		},
		{
			Repo: mockRepo2, Number: 11757, Title: "Modelagem do dispositivo",
			Author: "victoralcan", CreatedAt: ago(day(51)), UpdatedAt: ago(day(22)),
			IsDraft: true, Mergeable: model.MergeableConflict, BaseRef: "main", HeadRef: "feat/modelagem-dispositivo",
			Additions: 180, Deletions: 5, Changed: 10,
		},
		{
			Repo: mockRepo2, Number: 11748, Title: "feat: GestorController",
			Author: "Samuel-A-Santos", CreatedAt: ago(day(52)), UpdatedAt: ago(day(30)),
			IsDraft: true, Mergeable: model.MergeableConflict, BaseRef: "main", HeadRef: "feat/gestor-controller",
			Additions: 95, Deletions: 2, Changed: 6,
		},
	}

	runs := []model.WorkflowRun{
		{ID: 1, Repo: mockRepo, Name: "CI", Branch: "master", Event: "push", Actor: "alfoltran",
			Status: "completed", Conclusion: "failure", FailingJob: "build", FailingStep: "flyway:migrate",
			StartedAt: ago(40*time.Minute + 2*time.Minute + 29*time.Second), UpdatedAt: ago(40 * time.Minute),
			URL: "https://github.com/" + mockRepo + "/actions/runs/1"},
		{ID: 2, Repo: mockRepo, Name: "CI", Branch: "release/rubrica/esquema-empresa", Event: "pull_request",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(2*time.Hour + 32*time.Minute), UpdatedAt: ago(2 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/2"},
		{ID: 3, Repo: mockRepo, Name: "CI", Branch: "feat/progressao-desempenho-exercicio", Event: "pull_request",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(3*time.Hour + 23*time.Minute), UpdatedAt: ago(3 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/3"},
		{ID: 4, Repo: mockRepo, Name: "CI", Branch: "feature/concurso-lista-classificados", Event: "pull_request",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(4*time.Hour + 28*time.Minute), UpdatedAt: ago(4 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/4"},
		{ID: 5, Repo: mockRepo, Name: "Copilot Code Review", Branch: "master", Event: "push",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(2*time.Hour + 6*time.Minute), UpdatedAt: ago(2 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/5"},
		{ID: 6, Repo: mockRepo, Name: "Sync releases", Branch: "master", Event: "schedule",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(3*time.Hour + 40*time.Second), UpdatedAt: ago(3 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/6"},
		{ID: 7, Repo: mockRepo, Name: "DB migration", Branch: "master", Event: "workflow_dispatch",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(5*time.Hour + 8*time.Minute + 57*time.Second), UpdatedAt: ago(5 * time.Hour),
			URL: "https://github.com/" + mockRepo + "/actions/runs/7"},
		{ID: 8, Repo: mockRepo2, Name: "ETIPI Sync", Branch: "main", Event: "schedule",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(6*time.Hour + 16*time.Minute + 22*time.Second), UpdatedAt: ago(6 * time.Hour),
			URL: "https://github.com/" + mockRepo2 + "/actions/runs/8"},
	}
	for i := 0; i < 12; i++ {
		runs = append(runs, model.WorkflowRun{
			ID: int64(100 + i), Repo: mockRepo, Name: "CI", Branch: "master", Event: "push",
			Status: "completed", Conclusion: "success",
			StartedAt: ago(time.Duration(7+i)*time.Hour + 21*time.Minute),
			UpdatedAt: ago(time.Duration(7+i) * time.Hour),
			URL:       "https://github.com/" + mockRepo + "/actions/runs/100",
		})
	}

	issues := []model.Issue{
		{Repo: mockRepo, Number: 11890, Title: "Erro ao importar folha de dezembro"},
		{Repo: mockRepo, Number: 11402, Title: "Documentar o fluxo de rubricas"},
		{Repo: mockRepo2, Number: 902, Title: "Refatorar o cliente HTTP"},
	}
	people := []model.User{
		{Login: "alfoltran", Name: "Alexandre Foltran"},
		{Login: "fabiomiziara", Name: "Fabio Miziara"},
		{Login: "theorocha", Name: "Theo Rocha"},
		{Login: "victoralcan", Name: "Victor Alcantara"},
		{Login: MockViewer, Name: "D Reuse"},
	}
	return Snapshot{
		Viewer: MockViewer, PullRequests: prs, Runs: runs,
		Issues: issues, People: people,
	}, nil
}
