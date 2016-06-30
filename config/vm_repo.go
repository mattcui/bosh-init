package config

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type VMRepo interface {
	FindCurrent() (cid string, found bool, err error)
	FindCurrentIP() (ip string, found bool, err error)
	FindCurrentAgentId()(agentId string, found bool, err error)
	UpdateCurrent(cid string) error
	UpdateCurrentIP(ip string) error
	UpdateCurrentAgentId(agentId string) error
	ClearCurrent() error
}

type vMRepo struct {
	deploymentStateService DeploymentStateService
}

func NewVMRepo(deploymentStateService DeploymentStateService) VMRepo {
	return vMRepo{
		deploymentStateService: deploymentStateService,
	}
}

func (r vMRepo) FindCurrent() (string, bool, error) {
	deploymentState, err := r.deploymentStateService.Load()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Loading existing config")
	}

	currentVMCID := deploymentState.CurrentVMCID
	if currentVMCID != "" {
		return currentVMCID, true, nil
	}

	return "", false, nil
}

func (r vMRepo) UpdateCurrent(cid string) error {
	deploymentState, err := r.deploymentStateService.Load()
	if err != nil {
		return bosherr.WrapError(err, "Loading existing config")
	}

	deploymentState.CurrentVMCID = cid

	err = r.deploymentStateService.Save(deploymentState)
	if err != nil {
		return bosherr.WrapError(err, "Saving new config")
	}
	return nil
}


func (r vMRepo) FindCurrentIP() (string, bool, error) {
	deploymentState, err := r.deploymentStateService.Load()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Loading existing config")
	}

	currentIP := deploymentState.CurrentIP
	if currentIP != "" {
		return currentIP, true, nil
	}

	return "", false, nil
}

func (r vMRepo) UpdateCurrentIP (ip string) error {
	deploymentState, err := r.deploymentStateService.Load()
	if err != nil {
		return bosherr.WrapError(err, "Loading existing config")
	}

	deploymentState.CurrentAgentID= ip

	err = r.deploymentStateService.Save(deploymentState)
	if err != nil {
		return bosherr.WrapError(err, "Saving new config")
	}
	return nil
}

func (r vMRepo) FindCurrentAgentId() (string, bool, error) {
	deploymentState, err := r.deploymentStateService.Load()
	if err != nil {
		return "", false, bosherr.WrapError(err, "Loading existing config")
	}

	currentAgentID := deploymentState.CurrentAgentID
	if currentAgentID != "" {
		return currentAgentID, true, nil
	}

	return "", false, nil
}

func (r vMRepo) UpdateCurrentAgentId (agentId string) error {
	deploymentState, err := r.deploymentStateService.Load()
	if err != nil {
		return bosherr.WrapError(err, "Loading existing config")
	}

	deploymentState.CurrentAgentID= agentId

	err = r.deploymentStateService.Save(deploymentState)
	if err != nil {
		return bosherr.WrapError(err, "Saving new config")
	}
	return nil
}

func (r vMRepo) ClearCurrent() error {
	deploymentState, err := r.deploymentStateService.Load()
	if err != nil {
		return bosherr.WrapError(err, "Loading existing config")
	}

	deploymentState.CurrentVMCID = ""

	err = r.deploymentStateService.Save(deploymentState)
	if err != nil {
		return bosherr.WrapError(err, "Saving new config")
	}
	return nil
}
