pipeline {
    agent { label 'built-in' }

    options {
        timestamps()
        disableConcurrentBuilds()
        timeout(time: 60, unit: 'MINUTES')
        buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '10'))
    }

    environment {
        CI_CACHE = "${HOME}/.cache/soteria-ci"
    }

    stages {
        stage('Setup')   { steps { sh '.jenkins/scripts/setup.sh' } }
        stage('Version') {
            when { buildingTag() }
            steps { sh '.jenkins/scripts/version.sh' }
        }
        stage('Build macOS') {
            steps {
                sh '.jenkins/scripts/build-macos.sh arm64'
                sh '.jenkins/scripts/build-macos.sh amd64'
            }
        }
        stage('Build Windows') { steps { sh '.jenkins/scripts/build-windows.sh' } }
        stage('Checksums')     { steps { sh '.jenkins/scripts/checksums.sh' } }
        stage('Publish') {
            when { buildingTag() }
            environment { GH_TOKEN = credentials('github-token') }
            steps { sh '.jenkins/scripts/release.sh' }
        }
    }

    post {
        always  { archiveArtifacts artifacts: 'dist/*', allowEmptyArchive: true, fingerprint: true }
        cleanup { deleteDir() }
    }
}
