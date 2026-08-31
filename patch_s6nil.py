with open("internal/controller/consensuscapsule_controller.go", "r") as f:
    text = f.read()

replacement = """	if r.S6 == nil {
		return operatorv1alpha1.PhaseStopped
	}
	up, err := r.S6.IsUp(ctx, capsule)"""

text = text.replace("	up, err := r.S6.IsUp(ctx, capsule)", replacement)

with open("internal/controller/consensuscapsule_controller.go", "w") as f:
    f.write(text)
print("success")
