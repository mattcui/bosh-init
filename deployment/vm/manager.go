package vm

import (
	bicloud "github.com/cloudfoundry/bosh-init/cloud"
	biconfig "github.com/cloudfoundry/bosh-init/config"
	bideplmanifest "github.com/cloudfoundry/bosh-init/deployment/manifest"
	biagentclient "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-agent/agentclient"
	bihttpagent "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-agent/agentclient/http"
	bosherr "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/logger"
	biproperty "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/property"
	boshsys "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-init/internal/github.com/cloudfoundry/bosh-utils/uuid"
	bistemcell "github.com/cloudfoundry/bosh-init/stemcell"
)

type Manager interface {
	FindCurrent() (VM, bool, error)
	Create(bistemcell.CloudStemcell, bideplmanifest.Manifest) (VM, error)
}

type manager struct {
	vmRepo             biconfig.VMRepo
	stemcellRepo       biconfig.StemcellRepo
	diskDeployer       DiskDeployer
	agentClient        biagentclient.AgentClient
	agentClientFactory bihttpagent.AgentClientFactory
	cloud              bicloud.Cloud
	uuidGenerator      boshuuid.Generator
	fs                 boshsys.FileSystem
	logger             boshlog.Logger
	logTag             string
}

type softlayerVM struct {
	fullyQualifiedDomainName string
	primaryBackendIpAddress string
}

func NewManager(
	vmRepo biconfig.VMRepo,
	stemcellRepo biconfig.StemcellRepo,
	diskDeployer DiskDeployer,
	agentClient biagentclient.AgentClient,
	cloud bicloud.Cloud,
	uuidGenerator boshuuid.Generator,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) Manager {
	return &manager{
		cloud:         cloud,
		agentClient:   agentClient,
		vmRepo:        vmRepo,
		stemcellRepo:  stemcellRepo,
		diskDeployer:  diskDeployer,
		uuidGenerator: uuidGenerator,
		fs:            fs,
		logger:        logger,
		logTag:        "vmManager",
	}
}

func (m *manager) FindCurrent() (VM, bool, error) {
	vmCID, found, err := m.vmRepo.FindCurrent()
	if err != nil {
		return nil, false, bosherr.WrapError(err, "Finding currently deployed vm")
	}

	if !found {
		return nil, false, nil
	}

	vm := NewVM(
		vmCID,
		m.vmRepo,
		m.stemcellRepo,
		m.diskDeployer,
		m.agentClient,
		m.cloud,
		m.fs,
		m.logger,
	)

	return vm, true, err
}

func (m *manager) Create(stemcell bistemcell.CloudStemcell, deploymentManifest bideplmanifest.Manifest) (VM, error) {
	jobName := deploymentManifest.JobName()
	networkInterfaces, err := deploymentManifest.NetworkInterfaces(jobName)
	m.logger.Debug(m.logTag, "Creating VM with network interfaces: %#v", networkInterfaces)
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting network spec")
	}

	resourcePool, err := deploymentManifest.ResourcePool(jobName)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Getting resource pool for job '%s'", jobName)
	}

	agentID, err := m.uuidGenerator.Generate()
	if err != nil {
		return nil, bosherr.WrapError(err, "Generating agent ID")
	}

	cid, err := m.createAndRecordVm(agentID, stemcell, resourcePool, networkInterfaces)
	if err != nil {
		return nil, err
	}

	metadata := bicloud.VMMetadata{
		Deployment: deploymentManifest.Name,
		Job:        deploymentManifest.JobName(),
		Index:      "0",
		Director:   "bosh-init",
	}
	err = m.cloud.SetVMMetadata(cid, metadata)
	if err != nil {
		cloudErr, ok := err.(bicloud.Error)
		if ok && cloudErr.Type() == bicloud.NotImplementedError {
			//ignore it
		} else {
			return nil, bosherr.WrapErrorf(err, "Setting VM metadata to %s", metadata)
		}
	}

	m.logger.Debug(m.logTag, "WJQ: pre find_vm")

	hostname, privateIP, err = m.cloud.FindVM(cid)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Fetching details of vm: %s", cid)
	}
	softlayerVM := softlayerVM{fullyQualifiedDomainName: hostname, primaryBackendIpAddress: privateIP}
	if err := m.setupBoshInitEtcHosts(softlayerVM) ; err != nil {
		return nil, bosherr.WrapErrorf(err, "Writing to /etc/hosts")
	}

	m.logger.Debug(m.logTag, "WJQ Post find_vm")

	vm := NewVM(
		cid,
		m.vmRepo,
		m.stemcellRepo,
		m.diskDeployer,
		m.agentClient,
		m.cloud,
		m.fs,
		m.logger,
	)

	return vm, nil
}

func (m *manager) createAndRecordVm(agentID string, stemcell bistemcell.CloudStemcell, resourcePool bideplmanifest.ResourcePool, networkInterfaces map[string]biproperty.Map) (string, error) {
	cid, err := m.cloud.CreateVM(agentID, stemcell.CID(), resourcePool.CloudProperties, networkInterfaces, resourcePool.Env)
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Creating vm with stemcell cid '%s'", stemcell.CID())
	}

	// Record vm info immediately so we don't leak it
	err = m.vmRepo.UpdateCurrent(cid)
	if err != nil {
		return "", bosherr.WrapError(err, "Updating current vm record")
	}

	return cid, nil
}

func (m *manager) setupBoshInitEtcHosts(softlayerVM softlayerVM) (err error) {
	buffer := bytes.NewBuffer([]byte{})
	t := template.Must(template.New("etc-hosts").Parse(etcHostsTemplate))

	err = t.Execute(buffer, softlayerVM)
	if err != nil {
		return bosherr.WrapError(err, "Generating config from template")
	}

	err = p.fs.WriteFile("/etc/hosts", buffer.Bytes())
	if err != nil {
		return bosherr.WrapError(err, "Writing to /etc/hosts")
	}
	return nil
}

const etcHostsTemplate = `127.0.0.1 localhost
{{.primaryBackendIpAddress}} {{.fullyQualifiedDomainName}}
`
