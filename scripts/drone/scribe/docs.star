def build_documentation_pipeline():
  return {
    "kind": "pipeline",
    "type": "docker",
    "name": "build_documentation",
    "steps": [
      builtin_compile_pipeline_step(),
      download_grabpl_step(),
      yarn_install_step(),
      codespell_step(),
      lint_docs_step(),
      build_frontend_package_step(),
      build_frontend_documentation_step(),
      build_documentation_website_step(),
    ],
    "volumes": [
      {
        "name": "scribe",
        "temp": {
        },
      },
      {
        "name": "scribe-state",
        "temp": {
        },
      },
      {
        "name": "docker_socket",
        "host": {
          "path": "/var/run/docker.sock",
        },
      },
    ],
  }

def builtin_compile_pipeline_step():
  return {
    "command": [
      "go",
      "build",
      "-o",
      "/var/scribe/pipeline",
      "./pkg/build/ci",
    ],
    "environment": {
      "GOOS": "linux",
      "GOARCH": "amd64",
      "CGO_ENABLED": "0",
    },
    "image": "grafana/shipwright:go-v0.9.15",
    "name": "builtin-compile-pipeline",
    "volumes": [
      {
        "name": "scribe",
        "path": "/var/scribe",
      },
    ],
  }

def download_grabpl_step():
  return {
    "commands": [
      "/var/scribe/pipeline -step=1 -build-id=$DRONE_BUILD_NUMBER -state=file:///var/scribe-state/state.json -log-level=debug -version=v0.9.15 ./pkg/build/ci",
    ],
    "depends_on": [
      "builtin-compile-pipeline",
    ],
    "image": "grafana/shipwright:v0.9.15",
    "name": "download_grabpl",
    "volumes": [
      {
        "name": "scribe",
        "path": "/var/scribe",
      },
      {
        "name": "scribe-state",
        "path": "/var/scribe-state",
      },
    ],
  }

def yarn_install_step():
  return {
    "commands": [
      "/var/scribe/pipeline -step=2 -build-id=$DRONE_BUILD_NUMBER -state=file:///var/scribe-state/state.json -log-level=debug -version=v0.9.15 ./pkg/build/ci",
    ],
    "depends_on": [
      "download_grabpl",
    ],
    "image": "grafana/build-container:1.5.3",
    "name": "yarn_install",
    "volumes": [
      {
        "name": "scribe",
        "path": "/var/scribe",
      },
      {
        "name": "scribe-state",
        "path": "/var/scribe-state",
      },
    ],
  }

def codespell_step():
  return {
    "commands": [
      "/var/scribe/pipeline -step=5 -build-id=$DRONE_BUILD_NUMBER -state=file:///var/scribe-state/state.json -log-level=debug -version=v0.9.15 ./pkg/build/ci",
    ],
    "depends_on": [
      "yarn_install",
    ],
    "image": "grafana/build-container:1.5.3",
    "name": "codespell",
    "volumes": [
      {
        "name": "scribe",
        "path": "/var/scribe",
      },
      {
        "name": "scribe-state",
        "path": "/var/scribe-state",
      },
    ],
  }

def lint_docs_step():
  return {
    "commands": [
      "/var/scribe/pipeline -step=6 -build-id=$DRONE_BUILD_NUMBER -state=file:///var/scribe-state/state.json -log-level=debug -version=v0.9.15 ./pkg/build/ci",
    ],
    "depends_on": [
      "yarn_install",
    ],
    "image": "grafana/build-container:1.5.3",
    "name": "lint_docs",
    "volumes": [
      {
        "name": "yarn_cache",
      },
      {
        "name": "scribe",
        "path": "/var/scribe",
      },
      {
        "name": "scribe-state",
        "path": "/var/scribe-state",
      },
    ],
  }

def build_frontend_package_step():
  return {
    "commands": [
      "/var/scribe/pipeline -step=8 -build-id=$DRONE_BUILD_NUMBER -state=file:///var/scribe-state/state.json -log-level=debug -version=v0.9.15 -arg=github_token=$secret_github_token ./pkg/build/ci",
    ],
    "depends_on": [
      "codespell",
      "lint_docs",
    ],
    "environment": {
      "secret_github_token": "",
    },
    "image": "grafana/build-container:1.5.3",
    "name": "build_frontend_package",
    "volumes": [
      {
        "name": "scribe",
        "path": "/var/scribe",
      },
      {
        "name": "scribe-state",
        "path": "/var/scribe-state",
      },
    ],
  }

def build_frontend_documentation_step():
  return {
    "commands": [
      "/var/scribe/pipeline -step=9 -build-id=$DRONE_BUILD_NUMBER -state=file:///var/scribe-state/state.json -log-level=debug -version=v0.9.15 ./pkg/build/ci",
    ],
    "depends_on": [
      "build_frontend_package",
    ],
    "image": "grafana/build-container:1.5.3",
    "name": "build_frontend_documentation",
    "volumes": [
      {
        "name": "scribe",
        "path": "/var/scribe",
      },
      {
        "name": "scribe-state",
        "path": "/var/scribe-state",
      },
    ],
  }

def build_documentation_website_step():
  return {
    "commands": [
      "/var/scribe/pipeline -step=10 -build-id=$DRONE_BUILD_NUMBER -state=file:///var/scribe-state/state.json -log-level=debug -version=v0.9.15 ./pkg/build/ci",
    ],
    "depends_on": [
      "build_frontend_documentation",
    ],
    "image": "grafana/docs-base:latest",
    "name": "build_documentation_website",
    "volumes": [
      {
        "name": "scribe",
        "path": "/var/scribe",
      },
      {
        "name": "scribe-state",
        "path": "/var/scribe-state",
      },
    ],
  }

