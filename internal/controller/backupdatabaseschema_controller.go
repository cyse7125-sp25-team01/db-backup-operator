package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1 "csye7125-team01.store/backup-operator/api/v1"
)

// BackupDatabaseSchemaReconciler reconciles a BackupDatabaseSchema object
type BackupDatabaseSchemaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.csye7125-team01.store,resources=backupdatabaseschemas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.csye7125-team01.store,resources=backupdatabaseschemas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *BackupDatabaseSchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Starting reconciliation", "request", req)

	// Fetch the BackupDatabaseSchema instance
	var backup backupv1.BackupDatabaseSchema
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		log.Error(err, "Unable to fetch BackupDatabaseSchema")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Fetched BackupDatabaseSchema", "backup", backup)

		// Ensure ServiceAccount exists
	if err := r.ensureServiceAccount(ctx, backup.Spec.KubeServiceAccount, backup.Spec.GcpServiceAccount, "webapp"); err != nil {
		log.Error(err, "Failed to ensure ServiceAccount")
		return ctrl.Result{}, err
	}

	// Fetch the database password from the secret
	dbPasswordSecret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: backup.Spec.DbPasswordSecretNamespace,
		Name:      backup.Spec.DbPasswordSecretName,
	}
	log.Info("Fetching database password secret", "secretKey", secretKey)
	if err := r.Get(ctx, secretKey, dbPasswordSecret); err != nil {
		log.Error(err, "Unable to fetch database password secret")
		return ctrl.Result{}, err
	}

	dbPassword, exists := dbPasswordSecret.Data[backup.Spec.DbPasswordSecretKey]
	if !exists || len(dbPassword) == 0 {
		log.Error(fmt.Errorf("empty password"), "Database password is empty")
		return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("database password is empty")
	}
	log.Info("Successfully fetched database password")

	currentBackupJobName := fmt.Sprintf("backup-%s", time.Now().Format("20060102150405"))

	// Set the last backup job name (first run: "N/A", subsequent runs: previous job name)
	if backup.Status.LastBackupJobName == "" {
		backup.Status.LastBackupJobName = "N/A"
	}

	// Perform the backup
	log.Info("Starting backup process")
	gcsPath, err := r.performBackup(ctx, backup.Spec, string(dbPassword))
	if err != nil {
		log.Error(err, "Backup failed")
		backup.Status.BackupStatus = "Success"
	} else {
		log.Info("Backup completed successfully", "gcsPath", gcsPath)
		backup.Status.BackupStatus = "Success"
		backup.Status.BackupLocation = gcsPath
	}

	// Update the status of the BackupDatabaseSchema
	backup.Status.LastBackupTime = metav1.Now()
	previousJobName := backup.Status.LastBackupJobName
	backup.Status.LastBackupJobName = currentBackupJobName
	log.Info("Updating BackupDatabaseSchema status", "status", backup.Status)
	if err := r.Status().Update(ctx, &backup); err != nil {
		log.Error(err, "Unable to update BackupDatabaseSchema status")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	log.Info("Reconciliation completed successfully", "previousJob", previousJobName, "currentJob", currentBackupJobName)

	return ctrl.Result{RequeueAfter: 24 * time.Hour}, nil
}
// ensureServiceAccount ensures that the ServiceAccount exists and has the correct GCP annotation
func (r *BackupDatabaseSchemaReconciler) ensureServiceAccount(ctx context.Context, saName, gcpSA, namespace string) error {
	log := log.FromContext(ctx)

	// Check if ServiceAccount already exists
	existingSA := &corev1.ServiceAccount{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: saName}, existingSA)
	if err == nil {
		log.Info("ServiceAccount already exists", "name", saName)
		return nil
	}

	// Create a new ServiceAccount
	log.Info("Creating new ServiceAccount", "name", saName)
	newSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
			Annotations: map[string]string{
				"iam.gke.io/gcp-service-account": gcpSA,
			},
		},
	}

	if err := r.Create(ctx, newSA); err != nil {
		log.Error(err, "Failed to create ServiceAccount", "name", saName)
		return err
	}

	log.Info("Successfully created ServiceAccount", "name", saName)
	return nil
}


// performBackup creates a Kubernetes Job to perform the backup and upload it to GCS
func (r *BackupDatabaseSchemaReconciler) performBackup(ctx context.Context, spec backupv1.BackupDatabaseSchemaSpec, dbPassword string) (string, error) {
	log := log.FromContext(ctx)
	log.Info("Starting backup process", "dbHost", spec.DbHost, "dbPort", spec.DbPort, "dbUser", spec.DbUser, "dbName", spec.DbName, "dbSchema", spec.DbSchema)

	// Generate a timestamp for the backup file
	timestamp := time.Now().Format("20060102150405")
	backupFileName := fmt.Sprintf("backup_%s_%s_%s.sql", spec.DbName, spec.DbSchema, timestamp)
	gcsPath := fmt.Sprintf("gs://%s/%s", spec.GcsBucket, backupFileName)

	// Define the backup job with two containers and shared emptyDir volume
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("backup-%s", timestamp),
			Namespace: spec.BackupJobNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: spec.KubeServiceAccount,
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					Volumes: []corev1.Volume{
						{
							Name: "backup-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "pgdump-container",
							Image:   "alpine:latest", // Use Alpine with necessary tools installed
							Command: []string{"/bin/sh", "-c"},
							Args: []string{
								"apk add --no-cache postgresql-client && " +
									"pg_dump -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -n $DB_SCHEMA -f /tmp/$BACKUP_FILE && " +
									"echo 'Backup file created at /tmp/$BACKUP_FILE' && " +
									"ls -l /tmp && " +
									"sleep 20",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "PGPASSWORD",
									Value: dbPassword,
								},
								{
									Name:  "DB_HOST",
									Value: spec.DbHost,
								},
								{
									Name:  "DB_PORT",
									Value: fmt.Sprintf("%d", spec.DbPort),
								},
								{
									Name:  "DB_USER",
									Value: spec.DbUser,
								},
								{
									Name:  "DB_NAME",
									Value: spec.DbName,
								},
								{
									Name:  "DB_SCHEMA",
									Value: spec.DbSchema,
								},
								{
									Name:  "BACKUP_FILE",
									Value: backupFileName,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "backup-volume",
									MountPath: "/tmp",
								},
							},
						},
						{
							Name:    "gsutil-container",
							Image:   "google/cloud-sdk:alpine", // Google Cloud SDK image for gsutil
							Command: []string{"/bin/sh", "-c"},
							Args: []string{
								"echo 'Listing files in /tmp:' && " +
									"ls -l /tmp && " +
									"gsutil cp /tmp/$BACKUP_FILE $GCS_PATH && " +
									"echo 'Backup uploaded to GCS successfully' && " +
									"sleep 20",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "GCS_PATH",
									Value: gcsPath,
								},
								{
									Name:  "BACKUP_FILE",
									Value: backupFileName,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "backup-volume",
									MountPath: "/tmp",
								},
							},
						},
					},
				},
			},
			BackoffLimit: int32Ptr(2), // Retry up to 2 times
		},
	}

	// Create the backup job
	log.Info("Creating backup job", "namespace", spec.BackupJobNamespace, "job", job.Name)
	if err := r.Create(ctx, job); err != nil {
		log.Error(err, "Failed to create backup job")
		return "", fmt.Errorf("failed to create backup job: %v", err)
	}

	log.Info("Backup job created successfully", "gcsPath", gcsPath)
	return gcsPath, nil
}

// Helper function to get a pointer to an int32
func int32Ptr(i int32) *int32 {
	return &i
}

func (r *BackupDatabaseSchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1.BackupDatabaseSchema{}).
		Complete(r)
}
