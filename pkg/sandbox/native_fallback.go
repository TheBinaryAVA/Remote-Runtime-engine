//go:build !linux

package sandbox

import (
	"context"
	"fmt"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

// NativeCgroupV2Sandbox is a stub on non-Linux platforms.
type NativeCgroupV2Sandbox struct{}

func NewNativeCgroupV2Sandbox() *NativeCgroupV2Sandbox {
	return &NativeCgroupV2Sandbox{}
}

func (n *NativeCgroupV2Sandbox) Name() string {
	return "native_cgroupv2"
}

func (n *NativeCgroupV2Sandbox) IsAvailable() bool {
	return false
}

func (n *NativeCgroupV2Sandbox) Prepare(ctx context.Context, req *models.ExecutionRequest, lang languages.LanguageHandler) (*SandboxContext, error) {
	return nil, fmt.Errorf("native cgroup v2 sandbox is only supported on Linux hosts")
}

func (n *NativeCgroupV2Sandbox) Compile(ctx context.Context, sCtx *SandboxContext) (*models.CompilationResult, error) {
	return nil, fmt.Errorf("native cgroup v2 sandbox is only supported on Linux hosts")
}

func (n *NativeCgroupV2Sandbox) Execute(ctx context.Context, sCtx *SandboxContext) (*models.ExecutionResult, error) {
	return nil, fmt.Errorf("native cgroup v2 sandbox is only supported on Linux hosts")
}

func (n *NativeCgroupV2Sandbox) Cleanup(sCtx *SandboxContext) error {
	return nil
}
