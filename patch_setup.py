import re
with open("internal/controller/consensuscapsule_controller.go", "r") as f:
    text = f.read()

replacement = """	if r.subreconciler == nil {
		if r.S6 == nil && r.clientset != nil && r.restConfig != nil {
			r.S6 = consensus.NewS6Controller(r.clientset, r.restConfig)
		}
		r.subreconciler = consensus.NewSubReconcilersWithS6(r.Client, r.Scheme, r.Recorder, r.S6)
	}"""

pattern = r"\tif r\.subreconciler == nil \{\n\t\tif r\.S6 != nil \{\n\t\t\tr\.subreconciler = consensus\.NewSubReconcilersWithS6\(r\.Client, r\.Scheme, r\.Recorder, r\.S6\)\n\t\t\} else \{\n\t\t\tr\.subreconciler = consensus\.NewSubReconcilers\(r\.Client, r\.Scheme, r\.Recorder, r\.clientset, r\.restConfig\)\n\t\t\}\n\t\}"

if re.search(pattern, text):
    text = re.sub(pattern, replacement, text)
    with open("internal/controller/consensuscapsule_controller.go", "w") as f:
        f.write(text)
    print("success")
else:
    print("pattern not found")
