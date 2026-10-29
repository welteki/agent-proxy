{
  description = "Proxy that sends coding agents' model traffic through a gateway";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-26.05";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f (import nixpkgs {
        inherit system;
        overlays = [ self.overlays.default ];
      }));
    in
    {
      overlays.default = final: prev: {
        agent-proxy = final.buildGoModule {
          pname = "agent-proxy";
          version = "0-unstable-${self.shortRev or self.dirtyShortRev or "dirty"}";

          src = self;
          vendorHash = "sha256-duSLtifP+J3M9zCFHjjCtIeaRqvIJF6nNIlqEJYehfE=";

          env.CGO_ENABLED = 0;
          ldflags = [ "-s" "-w" ];

          meta = {
            description = "Proxy that sends coding agents' model traffic through a gateway";
            license = nixpkgs.lib.licenses.mit;
            mainProgram = "agent-proxy";
          };
        };
      };

      packages = forAllSystems (pkgs: {
        inherit (pkgs) agent-proxy;
        default = pkgs.agent-proxy;
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.gotools pkgs.gopls ];
        };
      });
    };
}
