package resolver

import (
	"fmt"
	"strings"
)

const (
	DefaultSource = "theopacks default"
)

type Resolver struct {
	packages         map[string]*RequestedPackage
	previousVersions map[string]string
}

type RequestedPackage struct {
	Name    string
	Version string
	Source  string

	IsVersionAvailable func(version string) bool
	SkipInstall        bool
}

type ResolvedPackage struct {
	Name             string  `json:"name"`
	RequestedVersion *string `json:"requestedVersion,omitempty"`
	ResolvedVersion  *string `json:"resolvedVersion,omitempty"`
	Source           string  `json:"source"`
}

type PackageRef struct {
	Name string
}

func NewRequestedPackage(name, defaultVersion string) *RequestedPackage {
	return &RequestedPackage{
		Name:    name,
		Version: defaultVersion,
		Source:  DefaultSource,
	}
}

func (p *RequestedPackage) SetVersion(version, source string) *RequestedPackage {
	p.Version = version
	p.Source = source
	return p
}

func NewResolver() *Resolver {
	return &Resolver{
		packages:         make(map[string]*RequestedPackage),
		previousVersions: make(map[string]string),
	}
}

func (r *Resolver) ResolvePackages() (map[string]*ResolvedPackage, error) {
	resolvedPackages := make(map[string]*ResolvedPackage)

	for name, pkg := range r.packages {
		fuzzyVersion := resolveToFuzzyVersion(pkg.Version)

		if pkg.IsVersionAvailable != nil {
			// When there's a custom validator, the fuzzy version must pass it
			if !pkg.IsVersionAvailable(fuzzyVersion) {
				return nil, fmt.Errorf("no version available for %s %s", name, pkg.Version)
			}
		}

		resolvedPkg := &ResolvedPackage{
			Name:             name,
			RequestedVersion: &pkg.Version,
			ResolvedVersion:  &fuzzyVersion,
			Source:           pkg.Source,
		}

		resolvedPackages[name] = resolvedPkg
	}

	return resolvedPackages, nil
}

func (r *Resolver) Get(name string) *RequestedPackage {
	return r.packages[name]
}

func (r *Resolver) Default(name, defaultVersion string) PackageRef {
	r.packages[name] = NewRequestedPackage(name, defaultVersion)

	if r.previousVersions[name] != "" && r.previousVersions[name] != defaultVersion {
		r.Version(PackageRef{Name: name}, r.previousVersions[name], "previous installed version")
	}

	return PackageRef{Name: name}
}

func (r *Resolver) Version(ref PackageRef, version, source string) PackageRef {
	if pkg, exists := r.packages[ref.Name]; exists {
		pkg.SetVersion(strings.TrimSpace(version), source)
	}
	return ref
}

func (r *Resolver) SetPreviousVersion(name, version string) {
	r.previousVersions[name] = version
}

func (r *Resolver) SetVersionAvailable(ref PackageRef, isVersionAvailable func(version string) bool) {
	r.packages[ref.Name].IsVersionAvailable = isVersionAvailable
}

func (r *Resolver) SetSkipInstall(ref PackageRef, skip bool) {
	r.packages[ref.Name].SkipInstall = skip
}
