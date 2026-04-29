// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package dotnet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseProject_TargetFramework(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.csproj": aspnetCsproj,
	})
	p, err := parseProject(a, "app.csproj")
	require.NoError(t, err)
	require.Equal(t, "net8.0", p.TargetFramework())
}

func TestParseProject_TargetFrameworks_PicksLast(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.csproj": `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFrameworks>net6.0;net8.0</TargetFrameworks>
  </PropertyGroup>
</Project>`,
	})
	p, err := parseProject(a, "app.csproj")
	require.NoError(t, err)
	require.Equal(t, "net8.0", p.TargetFramework())
}

func TestProject_IsAspNet_SdkAttr(t *testing.T) {
	a := createTempApp(t, map[string]string{"app.csproj": aspnetCsproj})
	p, err := parseProject(a, "app.csproj")
	require.NoError(t, err)
	require.True(t, p.IsAspNet())
}

func TestProject_IsAspNet_PackageRef(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.csproj": `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.App" Version="8.0.0" />
  </ItemGroup>
</Project>`,
	})
	p, err := parseProject(a, "app.csproj")
	require.NoError(t, err)
	require.True(t, p.IsAspNet())
}

func TestProject_IsAspNet_NotAspNet(t *testing.T) {
	a := createTempApp(t, map[string]string{"app.csproj": consoleCsproj})
	p, err := parseProject(a, "app.csproj")
	require.NoError(t, err)
	require.False(t, p.IsAspNet())
}

func TestProject_IsExecutable(t *testing.T) {
	cases := []struct {
		name   string
		csproj string
		exe    bool
	}{
		{"aspnet web sdk", aspnetCsproj, true},
		{"console exe", consoleCsproj, true},
		{"library", libraryCsproj, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, map[string]string{"app.csproj": tt.csproj})
			p, err := parseProject(a, "app.csproj")
			require.NoError(t, err)
			require.Equal(t, tt.exe, p.IsExecutable())
		})
	}
}

func TestTfmToVersion(t *testing.T) {
	cases := []struct{ in, out string }{
		{"net8.0", "8.0"},
		{"net6.0", "6.0"},
		{"netcoreapp3.1", "3.1"},
		{"net10.0", "10.0"},
		{"  net8.0  ", "8.0"},
	}
	for _, tt := range cases {
		require.Equal(t, tt.out, TfmToVersion(tt.in))
	}
}
