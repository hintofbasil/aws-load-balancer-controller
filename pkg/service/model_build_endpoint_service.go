package service

import (
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/annotations"

	"sigs.k8s.io/aws-load-balancer-controller/pkg/model/core"
	ec2model "sigs.k8s.io/aws-load-balancer-controller/pkg/model/ec2"
)

func (t *defaultModelBuildTask) buildEndpointService(ctx context.Context) error {
	acceptanceRequired, err := t.buildAcceptanceRequired(ctx)
	if err != nil {
		return err
	}
	// TODO figure out how to handle allowed Principles
	// allowedPrinciples, err := t.buildAllowedPrinciples(ctx)
	// if err != nil {
	// 	return err
	// }

	// TODO this throws a "LoadBalancer is not fulfilled yet" error
	// Try hard coding the LB ARN
	// loadBalancerArn, err := t.loadBalancer.LoadBalancerARN().Resolve(ctx)

	privateDNSName, err := t.buildPrivateDNSName(ctx)
	if err != nil {
		return err
	}
	tags, err := t.buildListenerTags(ctx)
	if err != nil {
		return err
	}
	spec := ec2model.VPCEndpointServiceSpec{
		AcceptanceRequired:      &acceptanceRequired,
		NetworkLoadBalancerArns: []core.StringToken{t.loadBalancer.LoadBalancerARN()},
		PrivateDNSName:          &privateDNSName,
		Tags:                    tags,
	}

	_ = ec2model.NewVPCEndpointService(t.stack, "TODO", spec)

	return nil
}

func (t *defaultModelBuildTask) buildAcceptanceRequired(_ context.Context) (bool, error) {
	rawAcceptanceRequired := ""
	_ = t.annotationParser.ParseStringAnnotation(annotations.SvcLBSuffixEndpointServiceAcceptanceRequired, &rawAcceptanceRequired, t.service.Annotations)
	// We could use strconv here but we want to be highly explicit
	switch rawAcceptanceRequired {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.Errorf("invalid service annotation %v, value must be one of [%v, %v]", annotations.SvcLBSuffixEndpointServiceAcceptanceRequired, true, false)
	}
}

func (t *defaultModelBuildTask) buildAllowedPrinciples(_ context.Context) ([]string, error) {
	var rawAllowedPrinciples []string
	_ = t.annotationParser.ParseStringSliceAnnotation(annotations.SvcLBSuffixEndpointServiceAllowedPrincipals, &rawAllowedPrinciples, t.service.Annotations)
	// TODO do we need to validate there is atleast one?
	// if rawAllowedPrinciples == "" {
	// 	return "", errors.Errorf("invalid service annotation %v, must not be empty", annotations.SvcLBSuffixEndpointServiceAllowedPrincipals)
	// }
	return rawAllowedPrinciples, nil
}

func (t *defaultModelBuildTask) buildPrivateDNSName(_ context.Context) (string, error) {
	rawPrivateDNSName := ""
	_ = t.annotationParser.ParseStringAnnotation(annotations.SvcLBSuffixEndpointServicePrivateDNSName, &rawPrivateDNSName, t.service.Annotations)
	if rawPrivateDNSName == "" {
		return "", errors.Errorf("invalid service annotation %v, must not be empty", annotations.SvcLBSuffixEndpointServicePrivateDNSName)
	}
	return rawPrivateDNSName, nil
}

// TODO handle SvcLBSuffixEndpointServicePrivateDNSName
