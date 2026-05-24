package instances

import (
	"encoding/json"
	"fmt"
	"mclauncher/configs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type LoaderType string

const (
	LoaderNone     LoaderType = "none"
	LoaderFabric   LoaderType = "fabric"
	LoaderForge    LoaderType = "forge"
	LoaderNeoForge LoaderType = "neoforge"
	LoaderQuilt    LoaderType = "quilt"
)

type Instance struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	MinecraftVersion string     `json:"minecraftVersion"`
	Loader           LoaderType `json:"loader"`
	LoaderVersion    string     `json:"loaderVersion"`
	GameDir          string     `json:"gameDir"`
	AllocMax         int        `json:"allocMax"`
	Fullscreen       bool       `json:"fullscreen"`
	ExtraJVMArgs     []string   `json:"extraJvmArgs"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type Store struct {
	SelectedInstanceID string     `json:"selectedInstanceId"`
	Instances          []Instance `json:"instances"`
}

type Manager struct {
	path         string
	instancesDir string
}

func NewManager(instancesDir string) *Manager {
	if strings.TrimSpace(instancesDir) == "" {
		instancesDir = configs.DefaultInstancesDir()
	}

	return &Manager{
		path:         filepath.Join(instancesDir, "instances.json"),
		instancesDir: instancesDir,
	}
}

func (m *Manager) Load() (*Store, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		store := &Store{}
		if err := m.ensureDefault(store); err != nil {
			return nil, err
		}
		_ = m.Save(store)
		return store, nil
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}

	if err := m.ensureDefault(&store); err != nil {
		return nil, err
	}

	return &store, nil
}

func (m *Manager) Save(store *Store) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return err
	}

	for i := range store.Instances {
		if store.Instances[i].ID == "" {
			store.Instances[i].ID = slug(store.Instances[i].Name)
		}

		if store.Instances[i].GameDir == "" {
			store.Instances[i].GameDir = filepath.Join(m.instancesDir, store.Instances[i].ID, ".minecraft")
		}

		if store.Instances[i].AllocMax <= 0 {
			store.Instances[i].AllocMax = 2048
		}

		store.Instances[i].UpdatedAt = time.Now()
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0644)
}

func (m *Manager) List() ([]Instance, error) {
	store, err := m.Load()
	if err != nil {
		return nil, err
	}

	return store.Instances, nil
}

func (m *Manager) Selected() (*Instance, error) {
	store, err := m.Load()
	if err != nil {
		return nil, err
	}

	for _, inst := range store.Instances {
		if inst.ID == store.SelectedInstanceID {
			return &inst, nil
		}
	}

	if len(store.Instances) == 0 {
		return nil, fmt.Errorf("žádná instance neexistuje")
	}

	return &store.Instances[0], nil
}

func (m *Manager) Select(id string) error {
	store, err := m.Load()
	if err != nil {
		return err
	}

	for _, inst := range store.Instances {
		if inst.ID == id {
			store.SelectedInstanceID = id
			return m.Save(store)
		}
	}

	return fmt.Errorf("instance %s neexistuje", id)
}

func (m *Manager) Create(name string, mcVersion string, loader LoaderType) (*Instance, error) {
	name = strings.TrimSpace(name)
	mcVersion = strings.TrimSpace(mcVersion)

	if name == "" {
		return nil, fmt.Errorf("název instance je prázdný")
	}

	if mcVersion == "" {
		mcVersion = "1.16.5"
	}

	store, err := m.Load()
	if err != nil {
		return nil, err
	}

	id := uniqueID(store, slug(name))
	now := time.Now()

	inst := Instance{
		ID:               id,
		Name:             name,
		MinecraftVersion: mcVersion,
		Loader:           loader,
		GameDir:          filepath.Join(m.instancesDir, id, ".minecraft"),
		AllocMax:         2048,
		Fullscreen:       false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	store.Instances = append(store.Instances, inst)
	store.SelectedInstanceID = inst.ID

	if err := os.MkdirAll(inst.GameDir, 0755); err != nil {
		return nil, err
	}

	if err := m.Save(store); err != nil {
		return nil, err
	}

	return &inst, nil
}

func (m *Manager) Delete(id string) error {
	store, err := m.Load()
	if err != nil {
		return err
	}

	var next []Instance
	found := false

	for _, inst := range store.Instances {
		if inst.ID == id {
			found = true
			continue
		}

		next = append(next, inst)
	}

	if !found {
		return fmt.Errorf("instance %s neexistuje", id)
	}

	store.Instances = next

	if store.SelectedInstanceID == id {
		store.SelectedInstanceID = ""
		if len(store.Instances) > 0 {
			store.SelectedInstanceID = store.Instances[0].ID
		}
	}

	return m.Save(store)
}

func (m *Manager) ensureDefault(store *Store) error {
	if len(store.Instances) > 0 {
		return nil
	}

	now := time.Now()
	id := "vanilla-1-16-5"

	inst := Instance{
		ID:               id,
		Name:             "Vanilla 1.16.5",
		MinecraftVersion: "1.16.5",
		Loader:           LoaderNone,
		GameDir:          filepath.Join(m.instancesDir, id, ".minecraft"),
		AllocMax:         2048,
		Fullscreen:       false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	store.Instances = []Instance{inst}
	store.SelectedInstanceID = id

	return os.MkdirAll(inst.GameDir, 0755)
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if s == "" {
		return "instance"
	}

	return s
}

func uniqueID(store *Store, base string) string {
	id := base

	for i := 2; ; i++ {
		exists := false

		for _, inst := range store.Instances {
			if inst.ID == id {
				exists = true
				break
			}
		}

		if !exists {
			return id
		}

		id = fmt.Sprintf("%s-%d", base, i)
	}
}
