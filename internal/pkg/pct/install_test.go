package pct_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/puppetlabs/pdkgo/internal/pkg/mock"
	"github.com/puppetlabs/pdkgo/internal/pkg/pct"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type InstallTest struct {
	name           string
	args           args
	expected       expected
	mocks          mocks
	mockReponses   mockReponses
	mockExecutions mockExecutions
}

// what goes in
type args struct {
	templatePath string
	targetDir    string
	gitUri       string
	force        bool
}

// what comes out
type expected struct {
	errorMsg string
	filepath string
}

// filesystem mocks
type mocks struct {
	dirs  []string
	files map[string]string
}

// package mock responses
type mockReponses struct {
	get    mock.GetResponse
	untar  []mock.UntarResponse
	gunzip []mock.GunzipResponse
}

type mockExecutions struct {
	name        string
	args        []string
	responseCmd *exec.Cmd
}

func TestInstall(t *testing.T) {

	templatePath := "path/to/somewhere"
	remoteTemplatPath := "https://somewhere.online/templates"
	extractionPath := "path/to/extract/to"
	tempWorkingPath := t.TempDir()

	tarballBytes := []byte{
		0x1F, 0x8B, 0x08, 0x08, 0xF7, 0x5E, 0x14, 0x4A, 0x00, 0x03, 0x67, 0x6F,
		0x6F, 0x64, 0x2D, 0x70, 0x72, 0x6F, 0x6A, 0x65, 0x63, 0x74, 0x2E, 0x74,
		0x61, 0x72, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00,
	}

	tests := []InstallTest{
		{
			name: "if it the file does not exist",
			args: args{
				templatePath: filepath.Join(templatePath, "good-project.tar.gz"),
				targetDir:    templatePath,
			},
			expected: expected{
				errorMsg: fmt.Sprintf("No template package at %v", filepath.Join(templatePath, "good-project.tar.gz")),
			},
		},
		{
			name: "should extract a tar.gz to a template folder",
			args: args{
				templatePath: filepath.Join(templatePath, "good-project.tar.gz"),
				targetDir:    extractionPath,
			},
			expected: expected{
				filepath: filepath.Join(extractionPath, "puppetlabs", "good-project", "1.0.0"),
			},
			mockReponses: mockReponses{
				untar: []mock.UntarResponse{
					{
						ReturnPath:  filepath.Join(extractionPath, "good-project"),
						ErrResponse: false,
					},
					{
						ReturnPath:  filepath.Join(extractionPath, "puppetlabs", "good-project", "1.0.0"),
						ErrResponse: false,
					},
				},
				gunzip: []mock.GunzipResponse{
					{
						Fail:     false,
						FilePath: filepath.Join(extractionPath, "good-project.tar"),
					},
				},
			},
			mocks: mocks{
				dirs: []string{
					templatePath,
					extractionPath,
					filepath.Join(extractionPath, "good-project"),
				},
				files: map[string]string{
					filepath.Join(templatePath, "good-project.tar.gz"): string(tarballBytes),
					filepath.Join(extractionPath, "good-project", "pct-config.yml"): `---
template:
  id: good-project
  author: puppetlabs
  version: 1.0.0
`,
				},
			},
		},
		{
			name: "if it fails to gunzip",
			args: args{
				templatePath: filepath.Join(templatePath, "good-project.tar.gz"),
				targetDir:    templatePath,
			},
			mockReponses: mockReponses{
				gunzip: []mock.GunzipResponse{
					{
						ErrResponse: true,
					},
				},
			},
			mocks: mocks{
				dirs: []string{
					templatePath,
				},
				files: map[string]string{
					filepath.Join(templatePath, "good-project.tar.gz"): string(tarballBytes),
				},
			},
			expected: expected{
				errorMsg: "gunzip error",
			},
		},
		{
			name: "if it fails to untar",
			args: args{
				templatePath: filepath.Join(templatePath, "good-project.tar.gz"),
				targetDir:    templatePath,
			},
			mocks: mocks{
				dirs: []string{
					templatePath,
				},
				files: map[string]string{
					filepath.Join(templatePath, "good-project.tar.gz"): string(tarballBytes),
				},
			},
			mockReponses: mockReponses{
				gunzip: []mock.GunzipResponse{
					{
						Fail:     false,
						FilePath: filepath.Join(extractionPath, "good-project.tar"),
					},
				},
				untar: []mock.UntarResponse{
					{
						ErrResponse: true,
					},
				},
			},
			expected: expected{
				errorMsg: "untar error",
			},
		},
		{
			name: "If the tarball doesnt contain an valid config",
			args: args{
				templatePath: filepath.Join(templatePath, "good-project.tar.gz"),
				targetDir:    extractionPath,
			},
			expected: expected{
				errorMsg: "Invalid config: open " + filepath.FromSlash("path/to/extract/to/good-project/pct-config.yml") + ": file does not exist",
			},
			mockReponses: mockReponses{
				untar: []mock.UntarResponse{
					{
						ReturnPath:  filepath.Join(extractionPath, "good-project"),
						ErrResponse: false,
					},
					{
						ReturnPath:  filepath.Join(extractionPath, "puppetlabs", "good-project", "1.0.0"),
						ErrResponse: false,
					},
				},
				gunzip: []mock.GunzipResponse{
					{
						Fail:     false,
						FilePath: filepath.Join(extractionPath, "good-project.tar"),
					},
				},
			},
			mocks: mocks{
				dirs: []string{
					templatePath,
					extractionPath,
					filepath.Join(extractionPath, "good-project"),
				},
				files: map[string]string{
					filepath.Join(templatePath, "good-project.tar.gz"): string(tarballBytes),
				},
			},
		},
		{
			name: "when the template already exists",
			args: args{
				templatePath: filepath.Join(templatePath, "good-project.tar.gz"),
				targetDir:    extractionPath,
			},
			expected: expected{
				filepath: filepath.Join(extractionPath, "puppetlabs", "good-project", "1.0.0"),
			},
			mockReponses: mockReponses{
				untar: []mock.UntarResponse{
					{
						ReturnPath:  filepath.Join(extractionPath, "good-project"),
						ErrResponse: false,
					},
					{
						ReturnPath:  filepath.Join(extractionPath, "puppetlabs", "good-project", "1.0.0"),
						ErrResponse: false,
					},
				},
				gunzip: []mock.GunzipResponse{
					{
						Fail:     false,
						FilePath: filepath.Join(extractionPath, "good-project.tar"),
					},
				},
			},
			mocks: mocks{
				dirs: []string{
					templatePath,
					extractionPath,
					filepath.Join(extractionPath, "good-project"),
					filepath.Join(extractionPath, "puppetlabs", "good-project", "1.0.0"),
				},
				files: map[string]string{
					filepath.Join(templatePath, "good-project.tar.gz"): string(tarballBytes),
					filepath.Join(extractionPath, "good-project", "pct-config.yml"): `---
template:
  id: good-project
  author: puppetlabs
  version: 1.0.0
`,
				},
			},
		},
		{
			name: "if the tar file is remote and does not exist",
			args: args{
				templatePath: fmt.Sprintf("%s/%s", remoteTemplatPath, "good-project.tar.gz"),
				targetDir:    templatePath,
			},
			expected: expected{
				errorMsg: fmt.Sprintf("Received response code 404 when trying to download from %s", fmt.Sprintf("%s/%s", remoteTemplatPath, "good-project.tar.gz")),
			},
			mockReponses: mockReponses{
				get: mock.GetResponse{
					RequestResponse: &http.Response{
						StatusCode: 404,
						// We still need the body to exist and be a reader, just with empty bytes
						Body: ioutil.NopCloser(bytes.NewReader([]byte{})),
					},
				},
			},
		},
		{
			name: "should download and extract a remote tar.gz to a template folder",
			args: args{
				templatePath: fmt.Sprintf("%s/%s", remoteTemplatPath, "good-project.tar.gz"),
				targetDir:    extractionPath,
			},
			expected: expected{
				filepath: filepath.Join(extractionPath, "puppetlabs", "good-project", "1.0.0"),
			},
			mockReponses: mockReponses{
				untar: []mock.UntarResponse{
					{
						ReturnPath:  filepath.Join(extractionPath, "good-project"),
						ErrResponse: false,
					},
					{
						ReturnPath:  filepath.Join(extractionPath, "puppetlabs", "good-project", "1.0.0"),
						ErrResponse: false,
					},
				},
				gunzip: []mock.GunzipResponse{
					{
						Fail:     false,
						FilePath: filepath.Join(extractionPath, "good-project.tar"),
					},
				},
				get: mock.GetResponse{
					RequestResponse: &http.Response{
						StatusCode: 200,
						Body:       ioutil.NopCloser(bytes.NewReader(tarballBytes)),
					},
				},
			},
			mocks: mocks{
				dirs: []string{
					templatePath,
					extractionPath,
					filepath.Join(extractionPath, "good-project"),
				},
				files: map[string]string{
					filepath.Join(templatePath, "good-project.tar.gz"): string(tarballBytes),
					filepath.Join(extractionPath, "good-project", "pct-config.yml"): `---
template:
  id: good-project
  author: puppetlabs
  version: 1.0.0
`,
				},
			},
		},
		{
			name: "if the git URI is invalid",
			args: args{
				gitUri: "invalid-uri",
			},
			expected: expected{
				errorMsg: "Could not parse template uri",
			},
		},
		{
			name: "if the URI is valid but does not direct to a git repository",
			args: args{
				gitUri: "http://example.com/templates",
			},
			expected: expected{
				errorMsg: "Could not clone git repository:",
				filepath: "",
			},
			mockExecutions: mockExecutions{
				name:        "git",
				args:        []string{"clone", "http://example.com/templates", filepath.Join(tempWorkingPath, "temp")},
				responseCmd: exec.Command("exit", "1"),
			},
		},
		{
			name: "if the URI is valid and does direct to a git repository",
			args: args{
				gitUri:    "https://github.com/puppetlabs/pct-test-template-01.git",
				targetDir: templatePath,
			},
			expected: expected{
				errorMsg: "",
				filepath: filepath.Join(templatePath, "test-user", "test-template", "0.1.0"),
			},
			mockExecutions: mockExecutions{
				name:        "git",
				args:        []string{"clone", "https://github.com/puppetlabs/pct-test-template-01.git", filepath.Join(tempWorkingPath, "temp")},
				responseCmd: exec.Command("echo", "test"),
			},
			mocks: mocks{
				dirs: []string{
					filepath.Join(tempWorkingPath, "temp"),
				},
				files: map[string]string{
					filepath.Join(tempWorkingPath, "temp", "pct-config.yml"): `---
template:
  id: test-template
  author: test-user
  version: 0.1.0
`,
				},
			},
		},
		{
			name: "if the git repository does not contain a 'pct-config.yml' file",
			args: args{
				gitUri:    "https://github.com/puppetlabs/pct-test-template-01.git",
				targetDir: templatePath,
			},
			expected: expected{
				errorMsg: "Invalid config:",
			},
			mockExecutions: mockExecutions{
				name:        "git",
				args:        []string{"clone", "https://github.com/puppetlabs/pct-test-template-01.git", filepath.Join(tempWorkingPath, "temp")},
				responseCmd: exec.Command("echo", "test"),
			},
			mocks: mocks{
				dirs: []string{
					filepath.Join(tempWorkingPath, "temp"),
				},
			},
		},
		{
			name: "if git is not installed",
			args: args{
				gitUri:    "https://github.com/puppetlabs/pct-test-template-01.git",
				targetDir: templatePath,
			},
			expected: expected{
				errorMsg: "Could not clone git repository:",
			},
			mockExecutions: mockExecutions{
				name:        "git",
				args:        []string{"clone", "https://github.com/puppetlabs/pct-test-template-01.git", filepath.Join(tempWorkingPath, "temp")},
				responseCmd: exec.Command("exit,", " 1"),
			},
			mocks: mocks{
				dirs: []string{
					filepath.Join(tempWorkingPath, "temp"),
				},
				files: map[string]string{
					filepath.Join(tempWorkingPath, "temp", "pct-config.yml"): `---
template:
  id: test-template
  author: test-user
  version: 0.1.0
`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fs := afero.NewMemMapFs()
			afs := &afero.Afero{Fs: fs}

			for _, path := range tt.mocks.dirs {
				afs.Mkdir(path, 0750) //nolint:gosec,errcheck // this result is not used in a secure application
			}

			for file, content := range tt.mocks.files {
				config, _ := afs.Create(file) //nolint:gosec,errcheck // this result is not used in a secure application
				config.Write([]byte(content)) //nolint:errcheck
			}

			installer := &pct.PctInstaller{
				Tar:        &mock.Tar{UntarResponse: tt.mockReponses.untar},
				Gunzip:     &mock.Gunzip{Fs: fs, GunzipResponse: tt.mockReponses.gunzip},
				AFS:        afs,
				IOFS:       &afero.IOFS{Fs: fs},
				HTTPClient: &mock.HTTPClient{RequestResponse: tt.mockReponses.get.RequestResponse},
				Exec:       &mock.Exec{ExpectedName: tt.mockExecutions.name, ExpectedArg: tt.mockExecutions.args, ResponseCmd: tt.mockExecutions.responseCmd},
			}

			var err error
			returnedPath := ""
			// Method of installation
			if tt.args.gitUri != "" {
				returnedPath, err = installer.InstallClone(tt.args.gitUri, tt.args.targetDir, tempWorkingPath, tt.args.force)
			} else {
				returnedPath, err = installer.Install(tt.args.templatePath, tt.args.targetDir, tt.args.force)
			}

			if tt.expected.errorMsg != "" && err != nil {
				assert.Contains(t, err.Error(), tt.expected.errorMsg)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected.filepath, returnedPath)
		})
	}
}
