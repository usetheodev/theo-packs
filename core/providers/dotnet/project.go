// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package dotnet

import (
	"encoding/xml"
	"strings"

	"github.com/usetheo/theopacks/core/app"
)

// Project represents the subset of a .NET project file we care about.
// Encoding/xml is loose enough to handle .csproj/.fsproj/.vbproj — they all
// share the same MSBuild XML schema for what we need.
type Project struct {
	XMLName        xml.Name        `xml:"Project"`
	Sdk            string          `xml:"Sdk,attr"`
	PropertyGroups []PropertyGroup `xml:"PropertyGroup"`
	ItemGroups     []ItemGroup     `xml:"ItemGroup"`
}

type PropertyGroup struct {
	TargetFramework  string `xml:"TargetFramework"`
	TargetFrameworks string `xml:"TargetFrameworks"`
	OutputType       string `xml:"OutputType"`
	AssemblyName     string `xml:"AssemblyName"`
}

type ItemGroup struct {
	PackageReferences []PackageReference `xml:"PackageReference"`
	Sdks              []SdkReference     `xml:"Sdk"`
}

type PackageReference struct {
	Include string `xml:"Include,attr"`
	Version string `xml:"Version,attr"`
}

type SdkReference struct {
	Name string `xml:"Name,attr"`
}

// parseProject reads a .csproj/.fsproj/.vbproj XML file. Unknown elements are
// ignored by the decoder, so future MSBuild additions don't break parsing.
func parseProject(a *app.App, path string) (*Project, error) {
	content, err := a.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := xml.Unmarshal([]byte(content), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// TargetFramework returns the first TFM the project targets. For multi-target
// frameworks (<TargetFrameworks>net6.0;net8.0</TargetFrameworks>), returns the
// LAST entry (most recent), which matches typical user intent.
func (p *Project) TargetFramework() string {
	for _, pg := range p.PropertyGroups {
		if pg.TargetFramework != "" {
			return pg.TargetFramework
		}
		if pg.TargetFrameworks != "" {
			parts := strings.Split(pg.TargetFrameworks, ";")
			if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
				return last
			}
		}
	}
	return ""
}

// TfmToVersion turns "net8.0" → "8.0", "netcoreapp3.1" → "3.1".
// .NET tag scheme is always major.minor.
func TfmToVersion(tfm string) string {
	tfm = strings.TrimSpace(tfm)
	tfm = strings.TrimPrefix(tfm, "netcoreapp")
	tfm = strings.TrimPrefix(tfm, "net")
	return tfm
}

// IsAspNet reports whether this project targets ASP.NET Core. Two signals
// suffice in practice:
//   - Sdk attribute equals "Microsoft.NET.Sdk.Web" (preferred form).
//   - Any PackageReference whose Include starts with "Microsoft.AspNetCore.".
func (p *Project) IsAspNet() bool {
	if strings.EqualFold(p.Sdk, "Microsoft.NET.Sdk.Web") {
		return true
	}
	for _, ig := range p.ItemGroups {
		for _, pr := range ig.PackageReferences {
			if strings.HasPrefix(pr.Include, "Microsoft.AspNetCore.") {
				return true
			}
		}
	}
	return false
}

// AssemblyName returns the explicit AssemblyName from the project file, or
// the empty string. The caller is expected to fall back to the project file
// stem (filename without extension).
func (p *Project) AssemblyName() string {
	for _, pg := range p.PropertyGroups {
		if pg.AssemblyName != "" {
			return pg.AssemblyName
		}
	}
	return ""
}

// IsExecutable reports whether this project produces a runnable binary.
// .NET executables have OutputType=Exe (or unset, since Exe is the default
// for Microsoft.NET.Sdk.Web). Library projects have OutputType=Library.
func (p *Project) IsExecutable() bool {
	if strings.EqualFold(p.Sdk, "Microsoft.NET.Sdk.Web") {
		return true
	}
	for _, pg := range p.PropertyGroups {
		ot := strings.TrimSpace(pg.OutputType)
		if strings.EqualFold(ot, "Exe") {
			return true
		}
		if strings.EqualFold(ot, "Library") {
			return false
		}
	}
	// Default for plain Microsoft.NET.Sdk projects is Library; we can't deploy.
	return false
}
