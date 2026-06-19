# KubeSwitcher Plugin: MCP

> [!IMPORTANT]
> This project is still work in progress and some errors as well as missing documentation are to be expected!

This is a plugin for the [kubeswitcher](https://github.com/Diaphteiros/kw) tool that allows to switch between clusters of an [openMCP](https://github.com/openmcp-project/openmcp-operator) landscape. Opposed to the [mcp plugin](https://github.com/Diaphteiros/kw_mcp), which does similar things, this one is designed for end-users and does not require any permissions that only landscape operators have.

## Installation

### Homebrew

```shell
brew tap diaphteiros/kubeswitcher
brew install kw_mcpu
```

### Manual Build
Clone the repository (including the submodule) and run
```shell
task install
```

> [!NOTE]
> This project uses [task](https://taskfile.dev/) instead of `make`.

## Configuration

The plugin takes a small configuration in the kubeswitcher config. The configuration is required for the plugin to work properly.
```yaml
<...>
- name: mcpu # under which kw subcommand this plugin will be reachable
  short: "Switch to mcp clusters (end-user)" # short message for display in 'kw --help'
  binary: kw_mcpu # name of or path to the plugin binary
  config:
    landscapes: # multiple openMCP landscapes can be specified, each is assumed to have its own onboarding cluster
      dev: # arbitrary identifier for the landscape
        onboardingKubeconfig:
          # exactly one of 'inline' and 'path' must be specified
          inline: | # inline kubeconfig
            apiVersion: v1
            kind: Config
            <...>
          path: path/to/kubeconfig # path to kubeconfig
        controlPlaneAccess: # see below for explanation
        - selectors:
            project:
              name: foo
            workspace:
              name: bar
            controlPlane:
              name: baz
          access:
            type: oidc
            name: foobar
        - selectors:
            project:
              matchLabels:
                example.com/has-foo-access: "true"
          access:
            type: token
            name: foo
        - selectors:
            controlPlane:
              names:
              - asdf
              - qwer
          access:
            type: oidc
            name: abc
        - access:
            type: oidc
            name: default
      production: # arbitrary landscape identifier
        <...>
```

### Landscape Onboarding Cluster Access

In order to interact with an openMCP landscape, access for the landscape's onboarding cluster needs to be configured. This can either be done inline, or by specifying a path to the onboarding cluster's kubeconfig.


### ControlPlane Cluster Access

How a `ControlPlane` can be accessed depends on its spec. In short, this `ControlPlane`
```yaml
apiVersion: core.open-control-plane.io/v2alpha1
kind: ControlPlane
metadata:
  name: test
  namespace: whatever
spec:
  iam:
    oidc:
      defaultProvider:
        roleBindings:
        - roleRefs:
          - kind: ClusterRole
            name: cluster-admin
          subjects:
          - kind: User
            name: john.doe@example.com
      extraProviders:
      - name: asdf
        <...>
    tokens:
    - name: foo
      roleRefs:
      - kind: ClusterRole
        name: cluster-admin
```
will receive a status which looks similar to this:
```yaml
status:
  access:
    oidc_asdf:
      name: oidc-asdf.test.kubeconfig
    oidc_mydefault:
      name: oidc-mydefault.test.kubeconfig
    token_foo:
      name: token-foo.test.kubeconfig
  conditions:
    <...>
  observedGeneration: 1
  phase: Ready
```

Each entry in `status.access` corresponds to one entry in either `spec.iam.oidc` or `spec.iam.tokens`.

> [!IMPORTANT]
> The name of the default OIDC provider depends on the landscape configuration. In this example, it was assumed to be `mydefault`.

For the tool to fetch the correct kubeconfig, it needs to be told which of the configured 'accesses' to use. This happens in `landscapes.<landscape-name>.controlPlaneAccess`. Each entry of this list follows this format:

```yaml
- selectors: # optional, specifies which controlplanes this configuration applies for
    project: # optional, makes the configuration apply to only the controlplanes whose projects match the selector
      name: foo # optional, matches only the project with this name
      names: # optional, matches all projects whose names are in the list
      - foo
      - bar
      matchLabels: # optional, standard k8s label selector
        foo.bar.baz/asdf: foobar
      matchExpressions: # optional, standard k8s label selector
      - key: foo.bar.baz/asdf
        operator: In
        values:
        - foobar
    workspace: # optional, makes the configuration apply to only the controlplanes whose workspaces match the selector
      <same as for project>
    controlPlane: # optional, makes the configuration apply to only the controlplanes which match the selector
      <same as for project>
  access:
    type: oidc # either 'oidc' or 'token'
    name: asdf # refers to name in ControlPlane spec
```

The `access` field is required and references the access from the `ControlPlane` to use. `name` refers to the access' name, while `type` specifies whether the access comes from an entry in `spec.iam.oidc` or `spec.iam.tokens` in the `ControlPlane`.

Selectors can be used to specify for which `ControlPlane` resources the respective access configuration should be used. Whenever the tool is used to gain access to a specific `ControlPlane`, the `controlPlaneAccess` list in the plugin configuration is evaluated from top to bottom and the first entry, whose selector matches the `ControlPlane` (and/or its `Project`/`Workspace`), determines which kubeconfig is used to access the `ControlPlane`. Here are a few details on how the selectors work:

- An empty selector matches everything.
- If multiple selectors are specified, they are ANDed, meaning the `ControlPlane` must match all of them for the entry to apply.
  - This also applies to the `name` and `names` fields of a specific selector. Basically, the `name` field only exists for convenience, when only a specific name should be matched, and the `names` field is for when not a single one but multiple names should be matched. Specifying both at the same time does not make sense at all, because the value of `name` would need to appear in `names` too, otherwise the selector will never match anything (and if it appears in both, the `name` field can be omitted).
- A selector which selects for workspaces with name `foo`, but does not specify anything for `project` and `controlplane` will match all `ControlPlane` resources that are in a workspace named `foo`, _independent of the project these workspaces - multiple projects might have workspaces with this name - are in_.
  - While there might be some cases in which this is useful, for most scenarios, a workspace selector will only be specified if a project one is specified, and a controlplane selector will only be specified if a workspace one is specified.

#### Example

As an example, take a look at the plugin configuration [above](#configuration).

- Excatly one specific `ControlPlane`, named `baz`, in a workspace named `bar`, in a project named `foo`, will be matched by the first entry.
  - For this `ControlPlane`, its OIDC access with name `foobar` will be used.
- All `ControlPlanes` in projects which have the `example.com/has-foo-access: "true"` label will be matched by the second entry.
  - If the `ControlPlane` is named `baz`, in a workspace named `bar`, in a project named `foo`, it will be matched by the first entry, because that one is earlier in the list.
  - For all matched `ControlPlane`s, their token-based access named `foo` will be used.
- `ControlPlane`s which are named `asdf` or `qwer` use an OIDC access named `abc`.
  - Unless they were already matched by one of the earlier two entries.
  - Only the name of the `ControlPlane` is relevant for this selector, not its project or workspace.
- The last entry in the list does not have a selector. It therefore matches all `ControlPlane`s which have not been matched by an earlier entry.
  - Uses OIDC access named `default`.

#### Fallback: Single Access

If the tool is used to access a `ControlPlane` and **none** of the entries in the plugin config's `controlPlaneAccess` list matches the specified `ControlPlane`, the tool checks whether the `ControlPlane` has **only a single access** configured (there is only one entry in the `ControlPlane`'s `status.access` map). If that is the case, then this access is used.

Note that this fallback only applies if no entry matched - if an entry matched, but the specified access does not exist on the `ControlPlane`, this will always result in an error.
