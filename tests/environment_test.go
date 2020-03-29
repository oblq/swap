package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/oblq/swap"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentHnadler(t *testing.T) {
	eh := swap.NewBuilder("").EnvHandler

	eh.SetCurrent(swap.DefaultEnvs.Local.Tag())
	require.Equal(t, swap.DefaultEnvs.Local, eh.Current())

	eh.SetCurrent(swap.DefaultEnvs.Production.Tag())
	require.Equal(t, swap.DefaultEnvs.Production, eh.Current())

	eh.SetCurrent(swap.DefaultEnvs.Staging.Tag())
	require.Equal(t, swap.DefaultEnvs.Staging, eh.Current())

	eh.SetCurrent(swap.DefaultEnvs.Testing.Tag())
	require.Equal(t, swap.DefaultEnvs.Testing, eh.Current())

	eh.SetCurrent(swap.DefaultEnvs.Development.Tag())
	require.Equal(t, swap.DefaultEnvs.Development, eh.Current())

	eh.SetCurrent("")
	_ = os.Setenv("BUILD_ENV", "")
	eh.Sources.Git = nil

	// helpers coverage
	println(eh.Current().Info())

	_ = os.Setenv("BUILD_ENV", "staging")
	require.Equal(t, swap.DefaultEnvs.Staging, eh.Current())

	eh.SetCurrent("")
	_ = os.Unsetenv("BUILD_ENV")

	eh.Sources.Git = swap.NewGitRepository("./")
	println(eh.Current().Info())

	eh.Sources.Git = nil
	require.Equal(t, eh.Current(), swap.DefaultEnvs.Testing,
		"Development is not testing by default during testing: "+eh.Current().Tag()+" - "+os.Args[0])

	// RegEx test
	testEnv := swap.NewEnvironment("branch/", `branch/*`)
	require.True(t, testEnv.MatchTag("branch/test"),
		"error in RegEx matcher...")

	testEnv = swap.NewEnvironment("test", `test*`)
	require.True(t, testEnv.MatchTag("test1"),
		"error in RegEx matcher...")

	eh.Sources.Git = swap.NewGitRepository("./")
}

func TestNewRepository(t *testing.T) {
	repo := swap.NewGitRepository("./")
	fmt.Println(repo.Info())
	require.NoError(t, repo.Error)
}

func TestNewWrongRepository(t *testing.T) {
	repo := swap.NewGitRepository("nonexistentFolder")
	require.Error(t, repo.Error)
}
