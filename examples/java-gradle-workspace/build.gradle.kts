plugins {
    id("org.springframework.boot") version "3.3.0" apply false
    id("io.spring.dependency-management") version "1.1.5" apply false
}

subprojects {
    apply(plugin = "java")

    // `apply(plugin = "java")` doesn't bring the type-safe `java {}` accessor
    // into the subprojects DSL scope; use configure<JavaPluginExtension> to
    // hit the same extension imperatively.
    configure<org.gradle.api.plugins.JavaPluginExtension> {
        toolchain {
            languageVersion = JavaLanguageVersion.of(21)
        }
    }

    repositories {
        mavenCentral()
    }
}
