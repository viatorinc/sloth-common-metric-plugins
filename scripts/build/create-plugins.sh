#!/bin/bash -eu
# quick hacky script to combined package and plugin files to ones that are working for sloth
# see project readme for more information


# current file path, ignoring symlinks
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

DEV_PLUGINS_FOLDER_NAME=dev-plugins
PLUGINS_FOLDERNAME=plugins
DEV_PLUGINS_PATH="$(cd "$SCRIPT_DIR/../../${DEV_PLUGINS_FOLDER_NAME}" && pwd)"
PLUGINS_PATH="$(cd "$SCRIPT_DIR/../../${PLUGINS_FOLDERNAME}" && pwd)"

# if we ever have more than one shared file we have to create a list
FILTER_TEMPLATE_MARKER="FILTER-TEMPLATE-LOCATION"
FILTER_IMPORT="request_elapsed_time_ms/filters"
FILTER_TEMPLATE="${DEV_PLUGINS_PATH}/${FILTER_IMPORT}/filters.go"
FILTER_PACKAGE_PREFIX="filters."

processFilterTemplate ()
{
  newFile="${1}"
  newFileTmp="${1}.tmp.go"

  # get package name
  grep -e "^package " "${newFile}" | head -n 1 > "${newFileTmp}"
  echo "import (" >> "${newFileTmp}"

  # create original file import list using marker
  origImports=$( sed '1,/import/d;/)/,$d' "${newFile}" | grep -v ${FILTER_IMPORT})

  # create template file import list using parenthesis
  templateImports=$(sed '1,/import/d;/)/,$d' "${FILTER_TEMPLATE}")

  echo -e "${origImports}\n" "${templateImports}" | sed 's~[ \t]*~~g' | sort -u >> "${newFileTmp}"
  echo ")" >> "${newFileTmp}"

  # insert FILTER_TEMPLATE

  sed '1,/'${FILTER_TEMPLATE_MARKER}'/d' "${FILTER_TEMPLATE}" >> "${newFileTmp}"

  sed -e '1,/'${FILTER_TEMPLATE_MARKER}'/d' -e 's~'${FILTER_PACKAGE_PREFIX}'~~g' "${newFile}" >> "${newFileTmp}"

  mv -f "${newFileTmp}" "${newFile}"

  go fmt "${newPath}" >/dev/null

  echo "processed '${newFile}'"
}

pushd ${DEV_PLUGINS_PATH} >/dev/null


# find plugin files (they have `func SLIPlugin(` code)
find .  -type f -name "*.go"  \
     -exec grep -q "^\s*func SLIPlugin" "{}" \; \
     -print \
   | sed 's~^./~~' \
   | while read plugin; do

  dir="${PLUGINS_PATH}/$(dirname ${plugin})"
  pluginFilename=$(basename ${plugin})
  pluginTestFilename=$(echo "${plugin}" | sed "s~.go$~_test.go~")
  mkdir -p "${dir}"

   # copy the file over
  newPath="${dir}/${pluginFilename}"
  cp -af "${plugin}" "${newPath}"
  cp -af "$(dirname ${plugin})/README.md" "${dir}/"

  # if a file requires additional processing then process it
  if $(grep -q "${FILTER_TEMPLATE_MARKER}" "${newPath}" ); then
    processFilterTemplate "${newPath}"
  fi

  # also process test files
  if [[ -f ${pluginTestFilename} ]];then
      newPluginTestFilename="${dir}/$(basename "${pluginTestFilename}")"
      # copy the testfile and fix the package path
      sed -e "s~/${DEV_PLUGINS_FOLDER_NAME}/~/${PLUGINS_FOLDERNAME}/~"  "${pluginTestFilename}" > ${newPluginTestFilename}
      if $(grep -q "${FILTER_TEMPLATE_MARKER}" "${newPluginTestFilename}" ); then
        processFilterTemplate "${newPluginTestFilename}"
      fi
  fi
  echo "copied dev-plugin '${plugin}' to '${dir}'"


done

rm -rf ${DEV_PLUGINS_PATH}

popd >/dev/null

