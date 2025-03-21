package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupDatabaseSchemaSpec defines the desired state of BackupDatabaseSchema
type BackupDatabaseSchemaSpec struct {
	DbHost                    string `json:"dbHost"`
	DbPort                    int    `json:"dbPort"`
	DbUser                    string `json:"dbUser"`
	DbName                    string `json:"dbName"`
	DbSchema                  string `json:"dbSchema"`
	DbPasswordSecretNamespace string `json:"dbPasswordSecretNamespace"`
	DbPasswordSecretName      string `json:"dbPasswordSecretName"`
	DbPasswordSecretKey       string `json:"dbPasswordSecretKey"`
	GcsBucket                 string `json:"gcsBucket"`
	KubeServiceAccount        string `json:"kubeServiceAccount"`
	GcpServiceAccount        string `json:"gcpServiceAccount"`
	BackupJobNamespace        string `json:"backupJobNamespace,"`
}

// BackupDatabaseSchemaStatus defines the observed state of BackupDatabaseSchema
type BackupDatabaseSchemaStatus struct {
	BackupStatus      string      `json:"backupStatus"`
	BackupLocation    string      `json:"backupLocation"`
	LastBackupTime    metav1.Time `json:"lastBackupTime"`
	LastBackupJobName string      `json:"lastBackupJobName"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BackupDatabaseSchema is the Schema for the backupdatabaseschemas API
type BackupDatabaseSchema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupDatabaseSchemaSpec   `json:"spec,omitempty"`
	Status BackupDatabaseSchemaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupDatabaseSchemaList contains a list of BackupDatabaseSchema
type BackupDatabaseSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupDatabaseSchema `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupDatabaseSchema{}, &BackupDatabaseSchemaList{})
}
