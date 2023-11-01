package files

// type FileStore struct {
// 	config *Config
// }

// func NewFilestore(config *Config) *FileStore {
// 	return &FileStore{
// 		config: config,
// 	}
// }

// func (f *FileStore) GetVolumeNames() []string {
// 	result := make([]string, len(f.config.Volumes))
// 	for idx, volume := range f.config.Volumes {
// 		result[idx] = volume.Name
// 	}
// 	return result
// }

// type VolumePathResult struct {
// 	Directories []string
// 	Files       []string
// }

// var ErrNoSuchVolume = errors.New("no such volume")
// var ErrBadPath = errors.New("bad path")

// func (f *FileStore) GetVolumePath(volume string, path string) (string, fs.FileInfo, []fs.FileInfo, error) {
// 	vol := f.config.GetVolume(volume)
// 	if vol == nil {
// 		return path, nil, nil, ErrNoSuchVolume
// 	}

// 	path, err := filepath.Abs(filepath.Join(vol.Path, path))
// 	if err != nil {
// 		return path, nil, nil, err
// 	}

// 	if !strings.HasPrefix(path, vol.Path) {
// 		return path, nil, nil, ErrBadPath
// 	}

// 	info, err := os.Stat(path)
// 	if err != nil {
// 		return path, nil, nil, err
// 	}

// 	if !info.IsDir() {
// 		return path, info, nil, nil
// 	}

// 	files, err := ioutil.ReadDir(path)
// 	if err != nil {
// 		return path, nil, nil, err
// 	}

// 	return path, nil, files, nil
// }
