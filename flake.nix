{
  description = "WireGuard DDNS - A lightweight tool that provides DDNS dynamic DNS support for WireGuard";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule rec {
          pname = "wg-ddns";
          version = "0.1.0";

          src = ./.;

          vendorHash = "sha256-wgZqHBHm0FqPr6UW4RbO8eFYJIDH/bOHfTVA6Q5Bc3Y=";

          nativeBuildInputs = with pkgs; [
            go
          ];

          # Generate Swagger docs before building;
          preBuild = ''
            export HOME=$TMPDIR
            export GOPATH=$TMPDIR/go
            export PATH=$GOPATH/bin:$PATH
            go install github.com/swaggo/swag/cmd/swag@latest
            swag init --parseDependency --parseInternal
          '';

          ldflags = [ "-s" "-w" ];

          installPhase = ''
            runHook preInstall
            mkdir -p $out/bin
            cp wg-ddns $out/bin/

            # Install systemd service files;
            mkdir -p $out/lib/systemd/system
            cp wg-ddns.service $out/lib/systemd/system/
            cp wg-ddns@.service $out/lib/systemd/system/
            runHook postInstall
          '';

          meta = with pkgs.lib; {
            description = "A lightweight tool that provides DDNS dynamic DNS support for WireGuard";
            homepage = "https://github.com/fernvenue/wg-ddns";
            license = licenses.gpl3Only;
            maintainers = [ ];
            platforms = platforms.linux ++ platforms.darwin;
            mainProgram = "wg-ddns";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gotools
            gopls
            delve
          ];
        };
      });
}