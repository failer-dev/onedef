package dart

import (
	"archive/zip"
	"bytes"

	"github.com/failer-dev/onedef/internal/meta"
)

func Generate(endpoints []meta.EndpointStruct, packageName string) ([]byte, error) {
	if packageName == "" {
		packageName = "onedef_sdk"
	}
	className := snakeToPascal(packageName)
	types := collectTypes(endpoints)

	modelsContent := generateModels(types)
	clientContent := "import 'dart:convert';\n\nimport 'package:http/http.dart' as http;\n\nimport 'models.dart';\n\n" +
		generateClient(endpoints, className)
	barrelContent := "export 'src/client.dart';\nexport 'src/models.dart';\n"
	pubspecContent := "name: " + packageName + "\nenvironment:\n  sdk: ^3.0.0\ndependencies:\n  http: ^1.2.0\n"

	return createZip(packageName, map[string]string{
		"lib/src/models.dart":          modelsContent,
		"lib/src/client.dart":          clientContent,
		"lib/" + packageName + ".dart": barrelContent,
		"pubspec.yaml":                 pubspecContent,
	})
}

func createZip(packageName string, files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for path, content := range files {
		f, err := w.Create(packageName + "/" + path)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
