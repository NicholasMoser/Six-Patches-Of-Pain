"""
Six Patches of Pain is an auto-updater for the Super Clash of Ninja 4 mod.
For more information see https://github.com/NicholasMoser/Six_Patches_Of_Pain
"""

import os
import sys
import json
import zlib
import subprocess
import platform
from shutil import which
import requests
from tqdm import tqdm

# folder for data files
DATA = 'data'

# xdelta binaries
XDELTA3_EXE = 'data/xdelta3.exe'
XDELTA3 = 'xdelta3'
XDELTA = 'xdelta'

# current version to see if a newer version exists
CURRENT_VERSION = 'data/current_version'

# git repo to download new releases from
GIT_REPOSITORY = 'data/git_repository'

# default git repository to download new releases from
DEFAULT_GIT_REPOSITORY = 'https://api.github.com/repos/NicholasMoser/SCON4-Releases/releases'

# the patch file to be downloaded
PATCH_FILE = 'data/patch'

# path of the GNT4 ISO if it's not in the current directory
GNT4_ISO_PATH = 'data/gnt4_iso_path'

# default name of the GNT4 iso if the user downloads it
DEFAULT_GNT4_ISO = 'data/GNT4.iso'

# Linux, Darwin, or Windows
PLATFORM = platform.system()

def main():
    """ Run the application. """
    print('Starting Six Patches of Pain...\n')
    verify_integrity()
    gnt4_iso = get_gnt4_iso()
    if gnt4_iso is None:
        fail('Unable to find vanilla GNT4 ISO to patch against.')
    new_version = download_new_version()
    patch_gnt4(gnt4_iso, 'SCON4-{}.iso'.format(new_version))
    set_current_version(new_version)
    if os.path.exists(PATCH_FILE):
        os.remove(PATCH_FILE)
    input("\nPress enter to exit...")

def verify_integrity():
    """ Verify the integrity of the auto-updater and required files. """
    # Make sure that the current working directory was not changed
    # This can occur when dragging and dropping an ISO in Windows
    os.chdir(os.path.dirname(sys.argv[0]))
    # Create data directory if it doesn't already exist
    if not os.path.exists(DATA):
        os.makedirs(DATA)
    # Check that xdelta3 exists
    if PLATFORM == 'Windows':
        if not os.path.exists(XDELTA3_EXE):
            msg = 'Unable to find xdelta3.exe in the data folder.\n'
            msg += 'Please verify that there is a folder named data with a file named xdelta3.exe\n'
            msg += 'If you do not see it, redownload Six Patches of Pain.\n'
            msg += 'It may also be an issue with your antivirus.'
            fail(msg)
    elif PLATFORM == 'Darwin':
        if which(XDELTA) is None:
            fail('Unable to find xdelta, please install xdelta.')
    elif which(XDELTA3) is None:
        fail('Unable to find xdelta3, please install xdelta3.')
    # If git repository is not set, set it to the default release repository
    if not os.path.exists(GIT_REPOSITORY):
        with open(GIT_REPOSITORY, 'w') as git_repo_file:
            git_repo_file.write(DEFAULT_GIT_REPOSITORY)
    # Delete any existing patch files, since they may be corrupted/old
    if os.path.exists(PATCH_FILE):
        os.remove(PATCH_FILE)

def get_gnt4_iso():
    """ Retrieves the vanilla GNT4 iso to patch against. """
    # First, check if it was drag and dropped onto the executable or provided as an arg
    if (len(sys.argv) > 1):
        dragged_path = sys.argv[1]
        if (os.path.exists(dragged_path)):
            if (is_gnt4(dragged_path)):
                set_gnt4_iso_path(dragged_path)
                return dragged_path
            else:
                print('Provided file is not a vanilla GNT4 ISO: ' + dragged_path)
        else:
            print('Provided path is not valid: ' + dragged_path)
    # Second, look for the iso in GNT4_ISO_PATH
    if os.path.exists(GNT4_ISO_PATH):
        with open(GNT4_ISO_PATH, 'r') as gnt4_iso_path_file:
            iso_path = gnt4_iso_path_file.read()
            if os.path.exists(iso_path):
                print('Found vanilla GNT4 at {}'.format(iso_path))
                return iso_path
    # Then, look for it recursively in the current directory
    for root, _, files in os.walk('.'):
        for curr_file in files:
            file_path = os.path.abspath(os.path.join(root, curr_file))
            if is_gnt4(file_path):
                set_gnt4_iso_path(file_path)
                print('Found vanilla GNT4 at {}'.format(file_path))
                return file_path
    # Last resort, query the user for its location
    while True:
        print('This updater requires a vanilla GNT4 ISO in order to auto-update.')
        print('Please do one of the following:')
        print('  1: Exit this application and drag your vanilla GNT4 ISO onto the executable')
        print('  2: Enter the file path to your local copy of vanilla GNT4 ISO')
        print('  3: Move a vanilla GNT4 ISO to the same folder as this program and restart')
        print('  4: Enter a link to a download for vanilla GNT4\n')
        user_input = input('Input: ')
        if os.path.exists(user_input):
            # Local file
            if is_gnt4(user_input):
                set_gnt4_iso_path(user_input)
                return user_input
            print('\nERROR: {} is not a clean vanilla GNT4 ISO'.format(user_input))
        else:
            # Download from interwebs
            file_path = try_to_download_gnt4(user_input)
            if os.path.exists(file_path):
                if is_gnt4(file_path):
                    set_gnt4_iso_path(file_path)
                    return file_path
                print('\nERROR: Downloaded file was not a vanilla GNT4 ISO.')
                os.remove(file_path)
    return None

def download_new_version():
    """ Download a new release if it exists and return the version name. """
    # Get the latest release
    with open(GIT_REPOSITORY, 'r') as git_repo_file:
        repo = git_repo_file.read()
    request = requests.get(repo)
    if request.status_code != 200:
        fail('Unable to access releases for {}\nStatus code: {}'.format(repo, request.status_code))
    releases = json.loads(request.text)
    if not releases:
        fail('No releases found at {}'.format(repo))
    latest_release = releases[0]
    # Stop if the latest release has already been patched locally
    latest_version = latest_release['name']
    if os.path.exists(CURRENT_VERSION):
        with open(CURRENT_VERSION, 'r') as current_version_file:
            current_version = current_version_file.read()
        if current_version == latest_version:
            fail('Already on latest version: {}'.format(latest_version))
    # Download the patch
    assets = latest_release['assets']
    if not assets:
        fail('No assets found in latest release for {}'.format(repo))
    elif len(assets) > 1:
        fail('Too many assets found in latest release for {}'.format(repo))
    download_url = assets[0]['browser_download_url']
    print('There is a new version of SCON4 available: {}'.format(latest_version))
    print('Downloading: {}'.format(latest_version))
    download(download_url, PATCH_FILE)
    return latest_version

def patch_gnt4(gnt4_iso, scon4_iso):
    """ Patches the given GNT4 ISO to the output SCON4 ISO path using the downloaded patch. """
    print('Patching GNT4...')
    if PLATFORM == 'Windows':
        xdelta = XDELTA3_EXE
    elif PLATFORM == 'Darwin':
        xdelta = XDELTA
    else:
        xdelta = XDELTA3
    args = [xdelta, '-f', '-d', '-s', gnt4_iso, PATCH_FILE, scon4_iso]
    output = subprocess.check_output(args)
    if output:
        print(output)
    if os.path.exists(scon4_iso) and os.stat(scon4_iso).st_size > 0:
        iso_full_path = os.path.abspath(scon4_iso)
        print('Patching complete. Saved to {}'.format(iso_full_path))
    else:
        fail('Failed to patch GNT4')

def is_gnt4(file_path):
    """ Returns whether or not the given file path is vanilla GNT4. """
    if file_path.lower().endswith('.iso'):
        with open(file_path, 'rb') as open_file:
            game_id = open_file.read(6)
        if game_id == b'G4NJDA':
            return hash_file(file_path) == '55EE8B1A'
    return False

def try_to_download_gnt4(url):
    """ Try to download the given GNT4 ISO and return empty string if it fails. """
    try:
        download(url, DEFAULT_GNT4_ISO)
        return os.path.join(os.getcwd(), DEFAULT_GNT4_ISO)
    except Exception as exception:
        print('Failed to download file with error: {}'.format(exception))
        return ''

def download(url, file_path):
    """ Download the file at the given URL to the file_path with a download status bar. """
    response = requests.get(url, stream=True)
    total_size_in_bytes = int(response.headers.get('content-length', 0))
    block_size = 1024
    progress_bar = tqdm(total=total_size_in_bytes, unit='iB', unit_scale=True)
    with open(file_path, 'wb') as file:
        for data in response.iter_content(block_size):
            progress_bar.update(len(data))
            file.write(data)
    progress_bar.close()
    if total_size_in_bytes not in (0, progress_bar.n):
        fail('Error downloading patch at: {}'.format(url))

def set_gnt4_iso_path(iso_path):
    """ Set the vanilla GNT4 ISO path to the vanilla GNT4 ISO path file. """
    with open(GNT4_ISO_PATH, 'w') as gnt4_iso_file:
        gnt4_iso_file.write(iso_path)

def set_current_version(version):
    """ Set the new version to the current version file. """
    with open(CURRENT_VERSION, 'w') as current_version_file:
        current_version_file.write(version)

def hash_file(file_path):
    """ Retrieves the CRC32 hash of a given file. """
    with open(file_path, 'rb') as open_file:
        current_hash = 0
        while True:
            buffer = open_file.read(65536)
            if not buffer:
                break
            current_hash = zlib.crc32(buffer, current_hash)
        return "%08X" % (current_hash & 0xFFFFFFFF)

def fail(message):
    """ Fail with the given message and prompt user to hit enter to exit. """
    print(message)
    if os.path.exists(PATCH_FILE):
        os.remove(PATCH_FILE)
    input("\nPress enter to exit...")
    sys.exit(1)

if __name__ == '__main__':
    main()
