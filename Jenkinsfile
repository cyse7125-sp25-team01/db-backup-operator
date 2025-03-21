pipeline {
    agent any

    stages {
	stage('Check Commit Message') {
            steps {
                script {
                    def commitMessage = sh(script: "git log -1 --pretty=%B", returnStdout: true).trim()
                    echo "Commit Message: ${commitMessage}"
                    sh """
                    echo '${commitMessage}' | npx commitlint --extends '@commitlint/config-conventional'
                    """
                }
            }
        }       
        stage('Helm Dependency Update') {
            steps {
                script {
                    def depUpdateResult = sh(script: "helm dependency update dist/chart/", returnStatus: true)
                    if (depUpdateResult != 0) {
                        error "Helm dependency update failed. Please check your chart dependencies."
                    }
                }
            }
        }

        stage('Helm Build (Package)') {
            steps {
                script {
                    def buildResult = sh(script: "helm dependency build dist/chart/", returnStatus: true)
                    if (buildResult != 0) {
                        error "Helm chart Build failed. Please check the chart."
                    }
                }
            }
        }
        stage('Helm Lint') {
            steps {
                script {
                    echo "Running Helm lint..."
                    def lintResult = sh(script: "helm lint dist/chart/", returnStatus: true)
                    if (lintResult != 0) {
                        error "Helm lint failed. Please fix the issues and try again."
                    }
                }
            }
        }

        stage('Helm Template') {
            steps {
                script {
                    echo "Running Helm template..."
                    def templateResult = sh(script: "helm template dist/chart/ > /dev/null", returnStatus: true)
                    if (templateResult != 0) {
                        error "Helm template failed. Please check the templates and try again."
                    }
                }
            }
        }
    }

    post {
        success {
            echo "Helm lint and template passed successfully!"
        }

        failure {
            echo "Pipeline failed."
        }
    }
}
