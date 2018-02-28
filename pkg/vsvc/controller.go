package vsvc

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/manifoldco/heighliner/pkg/api/v1alpha1"

	"github.com/jelmersnoeck/kubekit"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// Controller represents the VersionedMicroserviceController. This controller
// takes care of creating, updating and deleting lower level Kubernetese
// components that are associated with a specific VersionedMicroservice.
type Controller struct {
	rc        *rest.RESTClient
	cs        kubernetes.Interface
	namespace string
}

// NewController returns a new VersionedMicroservice Controller.
func NewController(cfg *rest.Config, cs kubernetes.Interface, namespace string) (*Controller, error) {
	rc, err := kubekit.RESTClient(cfg, &v1alpha1.SchemeGroupVersion, AddToScheme)
	if err != nil {
		return nil, err
	}

	return &Controller{
		cs:        cs,
		rc:        rc,
		namespace: namespace,
	}, nil
}

// Run runs the Controller in the background and sets up watchers to take action
// when the desired state is altered.
func (c *Controller) Run() error {
	log.Printf("Starting controller...")
	ctx, cancel := context.WithCancel(context.Background())

	go c.run(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Printf("Shutdown requested...")
	cancel()

	<-ctx.Done()
	log.Printf("Shutting down...")

	return nil
}

func (c *Controller) run(ctx context.Context) {
	watcher := kubekit.NewWatcher(
		c.rc,
		c.namespace,
		&CustomResource,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.onAdd,
			UpdateFunc: c.onUpdate,
			DeleteFunc: c.onDelete,
		},
	)

	go watcher.Run(ctx.Done())
}

func (c *Controller) onAdd(obj interface{}) {
	vsvc, ok := obj.(*v1alpha1.VersionedMicroservice)
	if !ok {
		log.Printf("Expected object to be of type `v1alpha1.VersionedMicroservice`")
		return
	}

	log.Printf("Deploying new application %s", vsvc.Name)
	dpl, err := GetDeployment(vsvc)
	if err != nil {
		log.Printf("Could not create Deployment: %s", err)
		return
	}

	if _, err := c.cs.Extensions().Deployments(vsvc.Namespace).Create(dpl); err != nil {
		log.Printf("Error deploying application '%s': %s", vsvc.Name, err)
		return
	}
}

func (c *Controller) onUpdate(old, new interface{}) {
	ovsvc, ok := old.(*v1alpha1.VersionedMicroservice)
	if !ok {
		log.Printf("Expected object to be of type `v1alpha1.VersionedMicroservice`")
		return
	}

	_, ok = old.(*v1alpha1.VersionedMicroservice)
	if !ok {
		log.Printf("Expected object to be of type `v1alpha1.VersionedMicroservice`")
		return
	}

	log.Printf("Updating application %s", ovsvc.Name)
	dpl, err := GetDeployment(ovsvc)
	if err != nil {
		log.Printf("Could not create Deployment: %s", err)
		return
	}

	if _, err := c.cs.Extensions().Deployments(ovsvc.Namespace).Update(dpl); err != nil {
		log.Printf("Error updating application '%s': %s", ovsvc.Name, err)
		return
	}
}

func (c *Controller) onDelete(obj interface{}) {
	vsvc, ok := obj.(*v1alpha1.VersionedMicroservice)
	if !ok {
		log.Printf("Expected object to be of type `v1alpha1.VersionedMicroservice`")
		return
	}

	log.Printf("Deleting application %s", vsvc.Name)
}