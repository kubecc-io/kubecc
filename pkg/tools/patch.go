package tools

import "github.com/banzaicloud/k8s-objectmatcher/patch"

const lastAppliedAnnotation = "kubecc.io/last-applied"

var (
	annotator  = patch.NewAnnotator(lastAppliedAnnotation)
	patchMaker = patch.NewPatchMaker(annotator)
)

var CalculatePatch = patchMaker.Calculate
var SetLastAppliedAnnotation = annotator.SetLastAppliedAnnotation
