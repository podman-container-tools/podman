package e2e_test

type rmMachine struct {
	/*
	  -f, --force           Stop and do not prompt before rming
	      --save-ignition   Do not delete ignition file
	      --save-image      Do not delete the image file

	*/
	force        bool
	saveIgnition bool
	saveImage    bool

	cmd []string
}

func (i *rmMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "rm"}
	if i.force {
		cmd = append(cmd, "--force")
	}
	if i.saveIgnition {
		cmd = append(cmd, "--save-ignition")
	}
	if i.saveImage {
		cmd = append(cmd, "--save-image")
	}
	cmd = append(cmd, m.name)
	i.cmd = cmd
	return cmd
}

func (i *rmMachine) withForce() *rmMachine {
	i.force = true
	return i
}

func (i *rmMachine) withSaveIgnition() *rmMachine {
	i.saveIgnition = true
	return i
}

func (i *rmMachine) withSaveImage() *rmMachine {
	i.saveImage = true
	return i
}
