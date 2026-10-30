package ui

func AskEC2(EC2InstancesName []string) (*string, error) {
	return askSelection("Choose EC2 Instance:", EC2InstancesName)
}
