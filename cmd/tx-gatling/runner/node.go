// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package runner

import (
	"log"
	"time"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
)

func UpNode() {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.BuildAndRunWithOptions("./Dockerfile", &dockertest.RunOptions{
		Name:       "shard-core-1",
		WorkingDir: "",
		Mounts: []string{

			"/opt/projects/work_jax/jax-core/tmp/core_1:/root/.jaxcore",
		},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"18444/tcp": {{HostIP: "", HostPort: "18444"}},
			"18334/tcp": {{HostIP: "", HostPort: "18334"}},
		},
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	if err := pool.Retry(func() error {
		return nil
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	time.Sleep(2 * time.Minute)
	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

}
