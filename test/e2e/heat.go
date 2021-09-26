package e2e

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"net/url"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/extensions/ec2credentials"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	"github.com/gophercloud/gophercloud/openstack/objectstorage/v1/containers"
	"github.com/gophercloud/gophercloud/openstack/orchestration/v1/stacks"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

//go:embed heat
var HeatTemplate embed.FS

var (
	keypairName   = "kubecc-e2e"
	containerName = "kubecc-e2e"
	stackName     = "kubecc-e2e"
	novaService   = "nova"
	region        = "RegionOne"
)

func getKeyPair(
	ctx context.Context,
	provider *gophercloud.ProviderClient,
) (*keypairs.KeyPair, error) {
	lg := meta.Log(ctx)
	lg.Info("Getting keypair")
	novaClient, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Name:   novaService,
		Region: region,
	})
	if err != nil {
		return nil, err
	}
	result := keypairs.Get(novaClient, keypairName, keypairs.GetOpts{})
	if result.Err != nil {
		lg.Info("Keypair not found, creating")
		cr := keypairs.Create(novaClient, keypairs.CreateOpts{
			Name: keypairName,
		})
		if cr.Err != nil {
			return nil, cr.Err
		}
		lg.Info("Keypair created")
		return cr.Extract()
	}
	return result.Extract()
}

func NewClient() (*gophercloud.ProviderClient, error) {
	return clientconfig.AuthenticatedClient(&clientconfig.ClientOpts{})
}

func getEC2Credentials(
	ctx context.Context,
	client *gophercloud.ProviderClient,
) (*ec2credentials.Credential, error) {
	lg := meta.Log(ctx)
	lg.Info("Getting EC2 credentials")
	identityClient, err := openstack.NewIdentityV3(client, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}
	token := client.GetAuthResult().(tokens.CreateResult)
	user, err := token.ExtractUser()
	if err != nil {
		return nil, err
	}
	project, err := token.ExtractProject()
	if err != nil {
		return nil, err
	}
	var credentials *ec2credentials.Credential
	userID := user.ID
	pager := ec2credentials.List(identityClient, userID)
	if err := pager.EachPage(func(p pagination.Page) (bool, error) {
		list, err := ec2credentials.ExtractCredentials(p)
		if err != nil {
			return false, err
		}
		for _, c := range list {
			if c.UserID == userID {
				lg.Info("Found existing EC2 credentials for user")
				credentials = &c
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return nil, err
	}
	if credentials == nil {
		lg.Info("No EC2 credentials found, creating")
		cr := ec2credentials.Create(identityClient, userID, ec2credentials.CreateOpts{
			TenantID: project.ID,
		})
		if cr.Err != nil {
			return nil, cr.Err
		}
		lg.Info("EC2 credentials created")
		return cr.Extract()
	}
	return credentials, nil
}

func CreateS3Bucket(
	ctx context.Context,
	provider *gophercloud.ProviderClient,
) (*S3Info, error) {
	lg := meta.Log(ctx)
	lg.Info("Creating S3 bucket")
	swiftClient, err := openstack.NewObjectStorageV1(provider, gophercloud.EndpointOpts{
		Region: region,
	})
	if err != nil {
		return nil, err
	}
	if r := containers.Get(swiftClient, containerName, containers.GetOpts{}); r.Err != nil {
		lg.Info("S3 bucket not found, creating")
		cr := containers.Create(swiftClient, containerName, containers.CreateOpts{})
		if cr.Err != nil {
			return nil, cr.Err
		}
		lg.Info("S3 bucket created")
	} else {
		// If the container already exists, delete and recreate it
		lg.Info("S3 bucket already exists, recreating")
		dr := containers.Delete(swiftClient, containerName)
		if dr.Err != nil {
			return nil, dr.Err
		}
		return CreateS3Bucket(ctx, provider)
	}

	credentials, err := getEC2Credentials(ctx, provider)
	if err != nil {
		lg.With(
			zap.Error(err),
		).Error("Failed to get EC2 credentials")
		return nil, err
	}
	url, err := url.Parse(swiftClient.Endpoint)
	if err != nil {
		return nil, err
	}
	return &S3Info{
		URL:       url.Host,
		Bucket:    containerName,
		AccessKey: credentials.Access,
		SecretKey: credentials.Secret,
	}, nil
}

type CreateStackResult struct {
	Stack *stacks.RetrievedStack
	Err   error
}

func CreateStack(
	ctx context.Context,
	provider *gophercloud.ProviderClient,
	stackName string,
) (chan CreateStackResult, error) {
	lg := meta.Log(ctx)
	lg.Info("Creating stack (timeout=60s)")
	keypair, err := getKeyPair(ctx, provider)
	if err != nil {
		lg.With(
			zap.Error(err),
		).Error("Failed to get keypair")
		return nil, err
	}
	heatClient, err := openstack.NewOrchestrationV1(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}
	tmplData, err := fs.ReadFile(HeatTemplate, "heat/k3s-e2e.yaml")
	if err != nil {
		return nil, err
	}
	cr := stacks.Create(heatClient, stacks.CreateOpts{
		Name: stackName,
		TemplateOpts: &stacks.Template{
			TE: stacks.TE{
				Bin: tmplData,
			},
		},
		Parameters: map[string]interface{}{
			"key_name": keypair.Name,
		},
		Timeout: 120,
	})
	if cr.Err != nil {
		lg.With(
			zap.Error(cr.Err),
		).Error("Failed to schedule stack for creation")
		return nil, cr.Err
	}
	lg.Info("Stack scheduled for creation")
	created, err := cr.Extract()
	if err != nil {
		return nil, err
	}
	resultC := make(chan CreateStackResult, 1)
	startTime := time.Now()
	go func() {
		report := 1
		defer close(resultC)
		for {
			r := stacks.Get(heatClient, stackName, created.ID)
			if r.Err != nil {
				resultC <- CreateStackResult{
					Err: r.Err,
				}
				return
			}
			stack, err := r.Extract()
			if err != nil {
				resultC <- CreateStackResult{
					Err: err,
				}
				return
			}
			switch stack.Status {
			case "CREATE_COMPLETE":
				resultC <- CreateStackResult{
					Stack: stack,
				}
			case "CREATE_IN_PROGRESS":
				if report%5 == 0 {
					lg.Infof("Stack creation in progress (elapsed: %ds)",
						int(time.Since(startTime).Round(time.Second).Seconds()))
				}
				report++
				time.Sleep(1 * time.Second)
			default:
				resultC <- CreateStackResult{
					Err: fmt.Errorf("Stack creation failed with status %s: %s",
						stack.Status, stack.StatusReason),
				}
				return
			}
		}
	}()
	return resultC, nil
}

type S3Info struct {
	URL       string
	Bucket    string
	AccessKey string
	SecretKey string
}

type TestInfra struct {
	Kubeconfig *api.Config
	S3Info     *S3Info
	Client     *gophercloud.ProviderClient
	Stack      *stacks.RetrievedStack
}

func SetupE2EInfra(ctx context.Context) (*TestInfra, error) {
	lg := meta.Log(ctx)
	lg.Info("Setting up E2E infrastructure")
	client, err := NewClient()
	if err != nil {
		return nil, err
	}
	s3Info, err := CreateS3Bucket(ctx, client)
	if err != nil {
		return nil, err
	}
	stackC, err := CreateStack(ctx, client, stackName)
	if err != nil {
		return nil, err
	}
	stack := <-stackC
	if stack.Err != nil {
		lg.With(
			zap.Error(stack.Err),
		).Error("Failed to create stack")
		return nil, stack.Err
	}
	lg.Info("Stack created successfully")
	var ip string
	var kubeconfig *api.Config

	for _, output := range stack.Stack.Outputs {
		if output["output_key"] == "control_plane_ip" {
			ip = output["output_value"].(string)
			break
		}
	}
	if ip == "" {
		return nil, fmt.Errorf("Could not find control_plane_ip in stack outputs")
	}
	lg.With(
		"ip", ip,
	).Info("Found control plane IP")
	for _, output := range stack.Stack.Outputs {
		if output["output_key"] == "kubeconfig" {
			data := []byte(output["output_value"].(string))
			jsonData := map[string]string{}
			if err := yaml.Unmarshal(data, &jsonData); err != nil {
				return nil, err
			}
			kubeconfigData := jsonData["1"]
			decoded, err := base64.StdEncoding.DecodeString(kubeconfigData)
			if err != nil {
				return nil, err
			}
			kubeconfig, err = clientcmd.Load(decoded)
			if err != nil {
				return nil, err
			}
			context := kubeconfig.Contexts[kubeconfig.CurrentContext]
			cluster := kubeconfig.Clusters[context.Cluster]
			cluster.Server = fmt.Sprintf("https://%s:6443", ip)
			break
		}
	}
	if kubeconfig == nil {
		return nil, fmt.Errorf("Could not find kubeconfig in stack outputs")
	}
	lg.Info("Found cluster kubeconfig")
	return &TestInfra{
		Kubeconfig: kubeconfig,
		S3Info:     s3Info,
		Client:     client,
		Stack:      stack.Stack,
	}, nil
}

func CleanupE2EInfra(ctx context.Context, infra *TestInfra) error {
	lg := meta.Log(ctx)
	lg.Info("Cleaning up E2E infrastructure")
	if infra.Stack == nil {
		return nil
	}
	heatClient, err := openstack.NewOrchestrationV1(infra.Client, gophercloud.EndpointOpts{})
	if err != nil {
		return err
	}
	if dr := stacks.Delete(heatClient, stackName, infra.Stack.ID); dr.Err != nil {
		lg.With(
			zap.Error(dr.Err),
		).Error("Failed to delete stack")
		return dr.Err
	}
	lg.Info("Stack deleted successfully")
	return nil
}
