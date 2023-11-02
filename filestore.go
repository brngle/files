package files

import (
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/dustin/go-humanize"
)

type FileStore struct {
	Volumes map[string]*Volume
}

type Volume struct {
	Name    string
	Path    string
	Privacy string

	Features map[string]struct{}
	UserIds  map[string]struct{}
}

func NewFileStore(config *Config) *FileStore {
	volumes := map[string]*Volume{}
	for _, volume := range config.Volumes {

		userIds := make(map[string]struct{})
		for _, roleName := range volume.Roles {
			for _, role := range config.Roles {
				if role.Name == roleName {
					for _, userId := range role.UserIds {
						userIds[userId] = struct{}{}
					}
				}
			}
		}

		features := make(map[string]struct{})
		for _, feature := range volume.Features {
			features[feature] = struct{}{}
		}

		if volume.Privacy == "" {
			volume.Privacy = "private"
		}

		volumes[volume.Name] = &Volume{
			Name:     volume.Name,
			Path:     volume.Path,
			Privacy:  volume.Privacy,
			Features: features,
			UserIds:  userIds,
		}
	}

	return &FileStore{
		Volumes: volumes,
	}
}

func (v *Volume) HasUserId(userId string) bool {
	_, ok := v.UserIds[userId]
	return ok
}

func (v *Volume) HasFeature(feature string) bool {
	_, ok := v.Features[feature]
	return ok
}

func (v *Volume) path(path string) (string, error) {
	return securejoin.SecureJoin(v.Path, path)
}

func (v *Volume) Stat(p string) (os.FileInfo, error) {
	path, err := v.path(p)
	if err != nil {
		return nil, err
	}
	return os.Stat(path)
}

func (v *Volume) Data(p string) ([]byte, error) {
	path, err := v.path(p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (v *Volume) Open(p string) (*os.File, error) {
	path, err := v.path(p)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (v *Volume) MkdirAll(p string, perm fs.FileMode) error {
	path, err := v.path(p)
	if err != nil {
		return err
	}
	return os.MkdirAll(path, perm)
}

func (v *Volume) Create(p string) (*os.File, error) {
	path, err := v.path(p)
	if err != nil {
		return nil, err
	}
	return os.Create(path)
}

type VolumeEntry struct {
	Name      string
	Path      string
	Size      int64
	HumanSize string
	IsDir     bool
	Type      string
}

func NewVolumeEntryFromStat(path string, info fs.FileInfo) *VolumeEntry {
	mtraw := mime.TypeByExtension(filepath.Ext(path))
	mimetype, _, _ := mime.ParseMediaType(mtraw)

	return &VolumeEntry{
		Name:      info.Name(),
		Path:      path,
		Size:      info.Size(),
		HumanSize: humanize.Bytes(uint64(info.Size())),
		IsDir:     info.IsDir(),
		Type:      mimetype,
	}
}

func (e *VolumeEntry) HasTag(tag string) bool {
	return hasMediaTag(e.Type, tag)
}

func (v *Volume) WalkDir(p string, fn func(string, fs.DirEntry, error) error) error {
	root, err := v.path(p)
	if err != nil {
		return err
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		return fn(filepath.Join(p, strings.TrimPrefix(path, root)), d, err)
	})
}

func (v *Volume) Entry(path string) (*VolumeEntry, error) {
	p, err := v.path(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	return NewVolumeEntryFromStat(path, info), nil
}

func (v *Volume) Entries(path string) ([]*VolumeEntry, error) {
	vpath, err := v.path(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(vpath)
	if err != nil {
		return nil, err
	}

	result := []*VolumeEntry{}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}

		result = append(result, NewVolumeEntryFromStat(filepath.Join(path, info.Name()), info))
	}

	sort.Slice(result, func(i, j int) bool {
		ii := result[i]
		jj := result[j]

		if ii.IsDir && !jj.IsDir {
			return true
		} else if !ii.IsDir && jj.IsDir {
			return false
		}

		return ii.Name < jj.Name
	})

	return result, nil
}
