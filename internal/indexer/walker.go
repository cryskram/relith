package indexer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"build":        true,
	"dist":         true,
	"out":          true,
	"bin":          true,
	"obj":          true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".env":         true,
	".mypy_cache":  true,
	".cache":       true,
	"__MACOSX":     true,
	".hg":          true,
	".svn":         true,
	".idea":        true,
	".vscode":      true,
	".DS_Store":    true,
	".sass-cache":  true,
	"coverage":     true,
	".next":        true,
	".nuxt":        true,
}

var binaryExts = map[string]bool{
	".o": true, ".so": true, ".dylib": true, ".dll": true,
	".exe": true, ".bin": true, ".out": true, ".a": true, ".lib": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".ico": true, ".webp": true, ".avif": true, ".tiff": true, ".tif": true,
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".aac": true, ".wma": true,
	".mp4": true, ".avi": true, ".mkv": true, ".mov": true, ".wmv": true, ".webm": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
	".pyc": true, ".pyo": true, ".class": true, ".jar": true, ".war": true,
	".deb": true, ".rpm": true, ".dmg": true, ".iso": true, ".img": true,
	".log": true, ".dump": true, ".core": true,
	".min.js": true, ".min.css": true,
}

type FileInfo struct {
	RelPath  string
	FullPath string
	Size     int64
	ModTime  int64
}

var highValueFiles = map[string]bool{
	"main.go":        true,
	"app.go":         true,
	"main.rs":        true,
	"lib.rs":         true,
	"index.js":       true,
	"index.ts":       true,
	"index.tsx":      true,
	"index.jsx":      true,
	"main.js":        true,
	"main.ts":        true,
	"main.py":        true,
	"__init__.py":    true,
	"package.json":   true,
	"tsconfig.json":  true,
	"go.mod":         true,
	"Cargo.toml":     true,
	"pyproject.toml": true,
}

func WalkRepo(rootPath string, maxFileSize int64) ([]FileInfo, error) {
	rootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	var high, normal []FileInfo
	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}

		if d.IsDir() {
			base := filepath.Base(rel)
			if skipDirs[base] {
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.Size() == 0 {
			return nil
		}
		if maxFileSize > 0 && info.Size() > maxFileSize {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(rel))
		if binaryExts[ext] {
			return nil
		}
		name := strings.ToLower(filepath.Base(rel))
		if strings.HasPrefix(name, ".") {
			return nil
		}

		files := &normal
		if highValueFiles[name] {
			files = &high
		}
		*files = append(*files, FileInfo{
			RelPath:  rel,
			FullPath: path,
			Size:     info.Size(),
			ModTime:  info.ModTime().Unix(),
		})
		return nil
	})

	return append(high, normal...), err
}

func ReadFileContent(path string, maxSize int64) (string, error) {
	if maxSize > 0 {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.Size() > maxSize {
			return "", nil
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
