package sshops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

type UploadRequest struct {
	HostID       string
	LocalPath    string
	RemotePath   string
	Timeout      time.Duration
	Overwrite    bool
	PreserveMode bool
	DryRun       bool
}

type UploadResult struct {
	HostID      string `json:"host"`
	LocalPath   string `json:"local_path"`
	RemotePath  string `json:"remote_path"`
	Files       int    `json:"files"`
	Bytes       int64  `json:"bytes"`
	Directories int    `json:"directories"`
	DurationMS  int64  `json:"duration_ms"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

type DownloadRequest struct {
	HostID       string
	RemotePath   string
	LocalPath    string
	Timeout      time.Duration
	Overwrite    bool
	PreserveMode bool
	DryRun       bool
}

type DownloadResult struct {
	HostID      string `json:"host"`
	RemotePath  string `json:"remote_path"`
	LocalPath   string `json:"local_path"`
	Files       int    `json:"files"`
	Bytes       int64  `json:"bytes"`
	Directories int    `json:"directories"`
	DurationMS  int64  `json:"duration_ms"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

func (s *Service) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	host, err := s.Host(req.HostID)
	if err != nil {
		return UploadResult{}, err
	}

	localPath := expandPath(req.LocalPath)
	info, err := os.Stat(localPath)
	if err != nil {
		return UploadResult{}, NewUserError("local_path_invalid", "failed to read local path", err)
	}

	if req.DryRun {
		return UploadResult{
			HostID:     host.ID,
			LocalPath:  localPath,
			RemotePath: req.RemotePath,
			DryRun:     true,
		}, nil
	}

	runCtx, cancel := s.operationContext(ctx, req.Timeout)
	defer cancel()

	startedAt := time.Now()
	client, err := dialSSH(host, s.cfg.Defaults.ConnectTimeoutSec)
	if err != nil {
		return UploadResult{}, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return UploadResult{}, NewUserError("sftp_failed", "failed to create SFTP client", err)
	}
	defer sftpClient.Close()

	remotePath := req.RemotePath
	if !info.IsDir() {
		remotePath, err = resolveRemoteFileTarget(sftpClient, req.RemotePath, filepath.Base(localPath))
		if err != nil {
			return UploadResult{}, err
		}
	}

	result := UploadResult{
		HostID:     host.ID,
		LocalPath:  localPath,
		RemotePath: remotePath,
	}

	if info.IsDir() {
		err = uploadDir(runCtx, sftpClient, localPath, remotePath, req.Overwrite, req.PreserveMode, &result)
	} else {
		result.Files = 1
		result.Bytes, err = uploadFile(runCtx, sftpClient, localPath, remotePath, info.Mode(), req.Overwrite, req.PreserveMode)
	}
	if err != nil {
		return UploadResult{}, err
	}

	result.DurationMS = time.Since(startedAt).Milliseconds()
	return result, nil
}

func (s *Service) Download(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
	host, err := s.Host(req.HostID)
	if err != nil {
		return DownloadResult{}, err
	}

	localPath := expandPath(req.LocalPath)
	if req.DryRun {
		return DownloadResult{
			HostID:     host.ID,
			RemotePath: req.RemotePath,
			LocalPath:  localPath,
			DryRun:     true,
		}, nil
	}

	runCtx, cancel := s.operationContext(ctx, req.Timeout)
	defer cancel()

	startedAt := time.Now()
	client, err := dialSSH(host, s.cfg.Defaults.ConnectTimeoutSec)
	if err != nil {
		return DownloadResult{}, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return DownloadResult{}, NewUserError("sftp_failed", "failed to create SFTP client", err)
	}
	defer sftpClient.Close()

	info, err := sftpClient.Stat(req.RemotePath)
	if err != nil {
		return DownloadResult{}, NewUserError("remote_path_invalid", "failed to stat remote path", err)
	}

	result := DownloadResult{
		HostID:     host.ID,
		RemotePath: req.RemotePath,
		LocalPath:  localPath,
	}

	if info.IsDir() {
		err = downloadDir(runCtx, sftpClient, req.RemotePath, localPath, req.Overwrite, req.PreserveMode, &result)
	} else {
		target, err := resolveLocalFileTarget(localPath, filepath.Base(req.RemotePath))
		if err != nil {
			return DownloadResult{}, err
		}
		result.LocalPath = target
		result.Files = 1
		result.Bytes, err = downloadFile(runCtx, sftpClient, req.RemotePath, target, info.Mode(), req.Overwrite, req.PreserveMode)
	}
	if err != nil {
		return DownloadResult{}, err
	}

	result.DurationMS = time.Since(startedAt).Milliseconds()
	return result, nil
}

func uploadDir(ctx context.Context, client *sftp.Client, localRoot, remoteRoot string, overwrite, preserveMode bool, out *UploadResult) error {
	return filepath.WalkDir(localRoot, func(currentPath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return NewUserError("timeout", "upload timed out", err)
		}

		rel, err := filepath.Rel(localRoot, currentPath)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = ""
		}
		remotePath := joinRemotePath(remoteRoot, filepath.ToSlash(rel))

		if entry.IsDir() {
			if err := client.MkdirAll(remotePath); err != nil {
				return NewUserError("sftp_failed", "failed to create remote directory", err)
			}
			out.Directories++
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		copied, err := uploadFile(ctx, client, currentPath, remotePath, info.Mode(), overwrite, preserveMode)
		if err != nil {
			return err
		}
		out.Files++
		out.Bytes += copied
		return nil
	})
}

func uploadFile(ctx context.Context, client *sftp.Client, localPath, remotePath string, mode os.FileMode, overwrite, preserveMode bool) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, NewUserError("timeout", "upload timed out", err)
	}
	if !overwrite {
		if _, err := client.Stat(remotePath); err == nil {
			return 0, NewUserError("overwrite_required", "remote file already exists; re-run with overwrite enabled", fmt.Errorf("%s", remotePath))
		}
	}

	if err := client.MkdirAll(path.Dir(remotePath)); err != nil {
		return 0, NewUserError("sftp_failed", "failed to create remote parent directory", err)
	}

	src, err := os.Open(localPath)
	if err != nil {
		return 0, NewUserError("local_path_invalid", "failed to open local file", err)
	}
	defer src.Close()

	flags := os.O_CREATE | os.O_WRONLY
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	dst, err := client.OpenFile(remotePath, flags)
	if err != nil {
		return 0, NewUserError("sftp_failed", "failed to open remote file", err)
	}
	defer dst.Close()

	copied, err := io.Copy(dst, src)
	if err != nil {
		return copied, NewUserError("sftp_failed", "failed to upload file", err)
	}
	if preserveMode {
		_ = client.Chmod(remotePath, mode.Perm())
	}
	return copied, nil
}

func downloadDir(ctx context.Context, client *sftp.Client, remoteRoot, localRoot string, overwrite, preserveMode bool, out *DownloadResult) error {
	walker := client.Walk(remoteRoot)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return NewUserError("sftp_failed", "failed while traversing remote directory", err)
		}
		if err := ctx.Err(); err != nil {
			return NewUserError("timeout", "download timed out", err)
		}

		rel := strings.TrimPrefix(walker.Path(), remoteRoot)
		rel = strings.TrimPrefix(rel, "/")
		localPath := filepath.Join(localRoot, filepath.FromSlash(rel))
		if walker.Stat().IsDir() {
			if err := os.MkdirAll(localPath, 0o755); err != nil {
				return NewUserError("local_path_invalid", "failed to create local directory", err)
			}
			out.Directories++
			continue
		}

		copied, err := downloadFile(ctx, client, walker.Path(), localPath, walker.Stat().Mode(), overwrite, preserveMode)
		if err != nil {
			return err
		}
		out.Files++
		out.Bytes += copied
	}
	return nil
}

func downloadFile(ctx context.Context, client *sftp.Client, remotePath, localPath string, mode os.FileMode, overwrite, preserveMode bool) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, NewUserError("timeout", "download timed out", err)
	}
	if !overwrite {
		if _, err := os.Stat(localPath); err == nil {
			return 0, NewUserError("overwrite_required", "local file already exists; re-run with overwrite enabled", fmt.Errorf("%s", localPath))
		}
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return 0, NewUserError("local_path_invalid", "failed to create local parent directory", err)
	}

	src, err := client.Open(remotePath)
	if err != nil {
		return 0, NewUserError("sftp_failed", "failed to open remote file", err)
	}
	defer src.Close()

	flags := os.O_CREATE | os.O_WRONLY
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	dst, err := os.OpenFile(localPath, flags, 0o644)
	if err != nil {
		return 0, NewUserError("local_path_invalid", "failed to open local file", err)
	}
	defer dst.Close()

	copied, err := io.Copy(dst, src)
	if err != nil {
		return copied, NewUserError("sftp_failed", "failed to download file", err)
	}
	if preserveMode {
		_ = os.Chmod(localPath, mode.Perm())
	}
	return copied, nil
}

func resolveRemoteFileTarget(client *sftp.Client, remotePath, localBase string) (string, error) {
	if strings.HasSuffix(remotePath, "/") {
		return joinRemotePath(remotePath, localBase), nil
	}
	info, err := client.Stat(remotePath)
	if err == nil && info.IsDir() {
		return joinRemotePath(remotePath, localBase), nil
	}
	return remotePath, nil
}

func resolveLocalFileTarget(localPath, remoteBase string) (string, error) {
	if strings.HasSuffix(localPath, string(os.PathSeparator)) {
		return filepath.Join(localPath, remoteBase), nil
	}
	info, err := os.Stat(localPath)
	if err == nil && info.IsDir() {
		return filepath.Join(localPath, remoteBase), nil
	}
	return localPath, nil
}
