plugins {
    `java-library`
    `maven-publish`
    signing

    kotlin("jvm") version Version.KOTLIN
}

repositories {
    mavenCentral()
}

dependencies {
    compileOnly("org.springframework.data:spring-data-mongodb:4.2.4")

    implementation("com.fasterxml.jackson.core:jackson-annotations:2.17.0")

//    implementation("org.springframework.boot:spring-boot-starter-data-mongodb:${Version.SPRING_BOOT}")
//    implementation(kotlin("stdlib"))
//    implementation("com.fasterxml.jackson.module:jackson-module-kotlin:2.15.2")
//    implementation("org.jetbrains.kotlin:kotlin-reflect")
//    implementation("org.jetbrains.kotlin:kotlin-stdlib-jdk8")
//
//    implementation("com.google.protobuf:protobuf-java:${Version.PROTOBUF}")
//    implementation("com.google.protobuf:protobuf-java-util:3.20.1")
//
//    compileOnly("org.bouncycastle:bcprov-${Version.BCPROV}")
//    compileOnly("org.bouncycastle:bcpkix-${Version.BCPROV}")
//    compileOnly("com.nimbusds:nimbus-jose-jwt:${Version.NUMBUS_JOSE_JWT}")
//
//    api(project(":intel-amt-feature-model"))
//    implementation("com.zeronsoftn.zerox.agent:zerox-agent-service-interface:${Version.ZEROX_AGENT}")
}

publishing {
    publications {
        create<MavenPublication>("maven") {
            from(components["java"])

            pom {
                name.set(project.name)
                description.set("opennoty")
                url.set("https://github.com/jc-lab/opennoty")
                licenses {
                    license {
                        name.set("The Apache License, Version 2.0")
                        url.set("http://www.apache.org/licenses/LICENSE-2.0.txt")
                    }
                }
                developers {
                    developer {
                        id.set("jclab")
                        name.set("Joseph Lee")
                        email.set("joseph@jc-lab.net")
                    }
                }
                scm {
                    connection.set("scm:git:https://github.com/jc-lab/opennoty.git")
                    developerConnection.set("scm:git:ssh://git@github.com/jc-lab/opennoty.git")
                    url.set("https://github.com/jc-lab/opennoty")
                }
            }
        }
    }
    repositories {
        maven {
            val releasesRepoUrl = "https://s01.oss.sonatype.org/service/local/staging/deploy/maven2/"
            val snapshotsRepoUrl = "https://s01.oss.sonatype.org/content/repositories/snapshots/"
            url = uri(if ("$version".endsWith("SNAPSHOT")) snapshotsRepoUrl else releasesRepoUrl)
            credentials {
                username = findProperty("ossrhUsername") as String?
                password = findProperty("ossrhPassword") as String?
            }
        }
    }
}

signing {
    useGpgCmd()
    sign(publishing.publications)
}

tasks.withType<Sign>().configureEach {
    onlyIf { project.hasProperty("signing.gnupg.keyName") || project.hasProperty("signing.keyId") }
}