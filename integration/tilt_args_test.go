//+build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTiltArgs(t *testing.T) {
	f := newFixture(t, "tilt_args")
	defer f.TearDown()

	f.tiltArgs = []string{"foo"}

	f.TiltWatch()
	require.NotZero(t, f.activeTiltUp.port)

	err := f.logs.WaitUntilContains("foo run", 5*time.Second)
	require.NoError(t, err)

	f.logs.Reset()

	// need to explicitly pass the port number to connect to the instance launched by this test
	err = f.tilt.Args([]string{"bar", fmt.Sprintf("--port=%d", f.activeTiltUp.port)}, f.LogWriter())
	if err != nil {
		// Currently, Tilt starts printing logs before the webserver has bound to a port.
		// If this happens, just sleep for a second and try again.
		duration := 2 * time.Second
		fmt.Printf("Error setting args. Sleeping (%s): %v\n", duration, err)

		time.Sleep(duration)
		err = f.tilt.Args([]string{"bar"}, f.LogWriter())
		require.NoError(t, err)
	}

	err = f.logs.WaitUntilContains("bar run", time.Second)
	require.NoError(t, err)

	require.NotContains(t, f.logs.String(), "foo run")
}
