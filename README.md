# Notebook Controller

Working on a remake of Kubeflow notebook controller/api-server. More specifically, I want to
improve the experience for both operations teams that manage the cluster, and the users that use
it.

## Goals
* Notebooks are created from a list of templates and configurations meant to work with those templates.
* Single ownership of resources (e.g. a user owns a notebook, not a profile)
* Dynamic RBAC rules based on notebook ownership (e.g. only the user that created the notebook can patch, update, or delete it)
* Only modify fields that the controller cares about -- this allows thinks mutating webhooks to modify resources
  maintained by the controller. If a user wants to add a header to a VirtualService by mutating webhook, they
  should be able to.
* Allow additional ports to be opened -- this could be done by webhook _if_ the controller didn't rewrite the whole
  spec during reconciliation.

## API

### Templates
A template is the base of a workload, like a notebook. It contains a full Pod spec, similar to a Deployment or StatefulSet,
but the template doesn't create anything.

```yaml
apiVersion: jackhoman.dev/v1beta1
kind: Template
metadata:
  name: jupyter-scipy
  namespace: kubeflow
spec:
  template:
    spec:
      containers:
      - name: main
        image: kubeflownotebookswg/jupyter-scipy:v1.7.0-rc.0
        ports:
        - containerPort: 8888
```

The template itself does nothing. It's just a Pod spec. It's used by a Notebook to create a workload.

```yaml
apiVersion: jackhoman.dev/v1beta1
kind: Notebook
metadata:
  name: jupyter-scipy
  namespace: default
spec:
  updatePolicy: Auto  # refresh from template when stopped 
  stopped: false # set this to true to stop the notebook
  templateRef:
    name: jupyter-scipy
    namespace: kubeflow
  resources:
    memory: 16Gi
    cpu: 4
```

For additional resource that need to be shared across templates, like configuring a database, s3 bucket,
package index or other resources, use a PodDefault.

```yaml
---
apiVersion: jackhoman.dev/v1beta1
kind: PodDefault
metadata:
  name: pypi.private.com
  namespace: kubeflow
spec:
    template:
      spec:
        containers:
        - name: main
          env:
          - name: PIP_INDEX_URL
            value: https://pypi.private.com
          volumeMounts:
          - mountPath: /var/run/secrets/pypi.private.com/ 
            name: pypi
        volumes:
        - name: pypi
          secret:
            secretName: pypi.private.com
---
apiVersion: jackhoman.dev/v1beta1
kind: PodDefault
metadata:
  name: spark-defaults
  namespace: kubeflow
spec:
  template:
    spec:
      containers:
        - name: main
          ports:
          - name: http-spark
            containerPort: 4040
---
apiVersion: jackhoman.dev/v1beta1
kind: Template
metadata:
  name: jupyter-scipy
  namespace: kubeflow
spec:
  template:
    spec:
      containers:
        - name: main
          image: kubeflownotebookswg/jupyter-scipy:v1.7.0-rc.0
          ports:
            - containerPort: 8888
  required:
  - name: pypi.private.com
  dependencies:
  # dependencies will be copied to the target namespace,
  # and will be owned by the revision.
  - apiVersion:  v1
    kind: Secret
    name: pypi.private.com
```

Rather than the single template model with a limited set of pod options, we can have many
templates with different images that are configured exactly for that particular image. There's
very little chance of misconfiguration this way.
