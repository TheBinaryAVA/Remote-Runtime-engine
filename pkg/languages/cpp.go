package languages

// CppHandler handles compilation and execution of C++ source files using g++.
type CppHandler struct{}

func (c *CppHandler) Name() string {
	return "cpp"
}

func (c *CppHandler) SourceFilename() string {
	return "solution.cpp"
}

func (c *CppHandler) BinaryFilename() string {
	return "solution"
}

func (c *CppHandler) NeedsCompilation() bool {
	return true
}

func (c *CppHandler) CompileCommand(sourcePath, binaryPath string) (string, []string) {
	// Standard speed-coding optimization & safety flags
	return "g++", []string{
		"-O3",
		"-std=c++17",
		"-Wall",
		"-Wextra",
		"-DONLINE_JUDGE",
		"-pipe",
		sourcePath,
		"-o",
		binaryPath,
	}
}

func (c *CppHandler) RunCommand(targetPath string) (string, []string) {
	return targetPath, []string{}
}
