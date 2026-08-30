package languages

// PythonHandler handles execution of Python 3 scripts.
type PythonHandler struct{}

func (p *PythonHandler) Name() string {
	return "python3"
}

func (p *PythonHandler) SourceFilename() string {
	return "solution.py"
}

func (p *PythonHandler) BinaryFilename() string {
	return ""
}

func (p *PythonHandler) NeedsCompilation() bool {
	return false
}

func (p *PythonHandler) CompileCommand(_, _ string) (string, []string) {
	return "", nil
}

func (p *PythonHandler) RunCommand(targetPath string) (string, []string) {
	// -u: unbuffered binary stdout and stderr
	// -B: don't write .pyc files on import
	return "python3", []string{
		"-u",
		"-B",
		targetPath,
	}
}
