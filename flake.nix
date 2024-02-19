{
  inputs.nixpkgs.url = "nixpkgs/nixpkgs-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }:
    let
      lastModifiedDate =
        self.lastModifiedDate or self.lastModified or "19700101";
      version = (builtins.substring 0 8 lastModifiedDate) + "-"
        + (if (self ? rev) then self.rev else "dirty");
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
      libFor = forAllSystems (system: import (nixpkgs + "/lib"));
      nixosLibFor = forAllSystems (system: import (nixpkgs + "/nixos/lib"));
    in flake-utils.lib.eachSystem supportedSystems (system:
      let
        pkgs = import nixpkgs { inherit system; };
        lib = import (nixpkgs + "/lib") { inherit system; };
        nixosLib = import (nixpkgs + "/nixos/lib") { inherit system; };
      in rec {
        devShells = let pkgs = nixpkgsFor.${system};
        in {
          default = pkgs.mkShell {
            nativeBuildInputs = with pkgs; [ pkg-config go ];
            buildInputs = with pkgs; [ nixfmt portaudio ];
          };
        };
        packages.default = pkgs.buildGoModule {
          inherit version;
          src = ./.;
          tags = [ "sdnotify" ];
          #vendorHash = pkgs.lib.fakeSha256;
          vendorHash =
            "sha256-EslwRP0hONNyK4QJDuENyuzUvmlFdfA+zsRywAbNfdk=";
          pname = "seekback";
          subPackages = [ "cmd/seekback" ];
          nativeBuildInputs = with pkgs; [ pkg-config ];
          buildInputs = with pkgs; [ portaudio ];
        };
        nixosModules.default = { config, lib, pkgs, ... }:
          with lib;
          with types;
          let cfg = config.seekback.services.seekback;
          in {
            options.seekback.services.seekback = {
              enable = mkEnableOption "the Seekback service";
              bufferSize = mkOption {
                type = int;
                default = 200000;
                description = "size of ring buffer in samples";
              };
              name = mkOption {
                type = path;
                default = "seekback-%%s.aiff";
                description =
                  "Template of path to save recordings to. %%s is replaced with the dump time (in RFC3339 format)";
              };
              latestName = mkOption {
                type = path;
                default = "seekback-latest.aiff";
                description = "Path to symlink to the latest dump.";
              };
            };
            config = mkIf cfg.enable {
              systemd.user.services.seekback = {
                Unit = {
                  Description = "Seekback: replay audio from the past";
                  StartLimitIntervalSec = 350;
                  StartLimitBurst = 30;
                };
                Service = {
                  ExecStart = "${
                      specialArgs.seekback.packages.${pkgs.system}.default
                    }/bin/seekback"
                    + " -buffer-size ${builtins.toString cfg.bufferSize}"
                    + " -name ${cfg.name}" + " -latest-name ${cfg.latestName}";
                  Restart = "on-failure";
                  RestartSec = 3;
                };
              };
            };
          };
      });
}
