#!/usr/bin/python3

# ------------------------------------------------------------------------------
# Name:		oxflasher.py
# Purpose:	Automated device flashing (based in ESP) to use with HAA and Tasmota
# Version:	v0.1.1
#
# Author:	David Alvarez Medina aka 0xDA_bit
# Mail:		oxdabit@protonmail.com
# Github:	OxDAbit
# Twitter:	@0xDA_bit
# Created:	08/02/2022
#
# Follow guide in: https://github.com/OxDAbit/oxflasher
#-------------------------------------------------------------------------------

# Import libraries
import os
import sys
import glob
import subprocess

from PyInquirer import prompt
from examples import custom_style_2
from prompt_toolkit.validation import Validator, ValidationError

# Create global variables
usb_ports = list()
flash_name = 'fullhaaboot.bin'
backup_name = 'backup.bin'
default_path = subprocess.check_output(['pwd']).decode().strip('\n')

# Colour Palette
class colors:
	reset='\033[0m'
	bold='\033[01m'
	disable='\033[02m'
	underline='\033[04m'
	reverse='\033[07m'
	strikethrough='\033[09m'
	invisible='\033[08m'
	class fg:
		black='\033[30m'
		red='\033[31m'
		green='\033[32m'
		orange='\033[33m'
		blue='\033[34m'
		purple='\033[35m'
		cyan='\033[36m'
		lightgrey='\033[37m'
		darkgrey='\033[90m'
		lightred='\033[91m'
		lightgreen='\033[92m'
		yellow='\033[93m'
		lightblue='\033[94m'
		pink='\033[95m'
		lightcyan='\033[96m'
	class bg:
		black='\033[40m'
		red='\033[41m'
		green='\033[42m'
		orange='\033[43m'
		blue='\033[44m'
		purple='\033[45m'
		cyan='\033[46m'
		lightgrey='\033[47m'

# Script action methods
class IndexValidation(Validator):
	def validate(self, document):
		try:
			int(document.text)
		except ValueError:
			raise ValidationError(message='Port index should be a number', cursor_position=len(document.text))
def usb_detection():
	global usb_ports

	if usb_ports: del usb_ports[:]
	for port in glob.glob('/dev/tty.*'): usb_ports.append(port)

	print('\n {}---- Ports detected -------------------------- '.format(colors.fg.lightgrey))
	for idx, port in enumerate(usb_ports):
		print('  {}{}{} - {}{}'.format(colors.fg.blue, idx, colors.fg.lightgrey, colors.fg.orange, port))
	print('{} ---------------------------------------------- \n'.format(colors.fg.lightgrey))

# UI Menus methods
def print_header():
	print('\n\t{}▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄'.format(colors.fg.blue, colors.fg.lightgrey))
	print('\t{}█▀▄▄▀█░█░█░▄▄█░██░▄▄▀█░▄▄█░████░▄▄█░▄▄▀'.format(colors.fg.blue, colors.fg.lightgrey))
	print('\t{}█░██░█▀▄▀█░▄██░██░▀▀░█▄▄▀█░▄▄░█░▄▄█░▀▀▄'.format(colors.fg.blue, colors.fg.lightgrey))
	print('\t{}██▄▄██▄█▄█▄███▄▄█▄██▄█▄▄▄█▄██▄█▄▄▄█▄█▄▄'.format(colors.fg.blue, colors.fg.lightgrey))
	print('\t{}▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀\n'.format(colors.fg.blue, colors.fg.lightgrey))

	print('\tby {}0XDΛ_BIƬ\n'.format(colors.fg.yellow))
def menu_list_ports():
	os.system('clear')
	print_header()
	usb_detection()
	answers = prompt(list_questions, style=custom_style_2)
	return answers
def menu_select_action(usb):
	os.system('clear')
	print_header()
	print('{}> {}USB port {}{}{} had been selected\n'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.lightblue, usb, colors.fg.lightgrey))
	answers = prompt(action_questions, style=custom_style_2)

	return answers
def menu_backup(usb):
	global backup_name, default_path

	os.system('clear')

	print_header()
	print('{}> {}The default path is: {}{}'.format(colors.fg.blue, colors.fg.lightgrey, colors.fg.yellow, default_path))
	print('{}> {}The default name is: {}{}'.format(colors.fg.blue, colors.fg.lightgrey, colors.fg.yellow, backup_name))
	print('\n{}! Leave fields empty if you want to use the default path, name or both of them !\n'.format(colors.fg.orange))

	answers = prompt(backup_questions, style=custom_style_2)
	if answers['path']:
		default_path = answers['path'].strip('\n')
		if default_path.endswith('/'): default_path = default_path[0:len(default_path)-1]
	if answers['name']:
		backup_name = answers['name'].strip('\n')
		if backup_name.startswith('/'): backup_name = backup_name[1:]
		if not backup_name.endswith('.bin'): backup_name = '{}.bin'.format(backup_name)

	print('\n{}> {}Creating backup:\n\t- Backup: {}{}/{} {}\n\t- USB port {}{}\n{}'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.lightblue, default_path, backup_name, colors.fg.lightgrey, colors.fg.lightblue, usb, colors.fg.green))

	answers = prompt(confirm_questions, style=custom_style_2)
	if answers['confirmation'] == True:
		if os.path.exists('{}'.format(default_path)):
			print('\n{}> {}Starting backup process\n{}'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.green))
			try:
				os.system('esptool.py -p {} read_flash 0x00000 0x100000 {}/{}'.format(usb, default_path, backup_name))
				print('\n{}> {}Backup created {}SUCCESSFULLY{} in{} {}/{}{}\n'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.green, colors.fg.lightgrey, colors.fg.cyan, default_path, backup_name, colors.fg.lightgrey))
			except Exception as e: print('{}[ERROR]{} An error occurred during the backup process. Error information: {}{}{}\n'.format(colors.fg.red, colors.fg.orange, colors.fg.lightred, e, colors.fg.lightgrey))
		else: print("\n{}Path file doesn't exist. Aborting backup process{}\n".format(colors.fg.orange, colors.fg.lightgrey))
	else: print("\n{}! Backup process canceled !{}\n".format(colors.fg.orange, colors.fg.lightgrey))
def menu_erase(usb):
	os.system('clear')
	print_header()
	print('\n{}! ERASE process will be executed in usb {} !\n'.format(colors.fg.orange, usb))
	answers = prompt(erase_questions, style=custom_style_2)
	if answers['erase_confirmation'] == True:
		print('\n{}> {}Starting erase process\n{}'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.green))
		try:
			os.system('esptool.py -p {} erase_flash'.format(usb))
			print('\n{}> {}Erase process complete {}SUCCESSFULLY{}\n'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.green, colors.fg.lightgrey))
		except Exception as e: print('{}[ERROR]{} An error occurred during the erase process. Error information: {}{}{}\n'.format(colors.fg.red, colors.fg.orange, colors.fg.lightred, e, colors.fg.lightgrey))
	else: print("\n{}! Erase process canceled !{}\n".format(colors.fg.orange, colors.fg.lightgrey))
def menu_flash(usb):
	global flash_name, default_path

	os.system('clear')

	print_header()
	print('{}> {}The default path is: {}{}'.format(colors.fg.blue, colors.fg.lightgrey, colors.fg.yellow, default_path))
	print('{}> {}The default name is: {}{}'.format(colors.fg.blue, colors.fg.lightgrey, colors.fg.yellow, flash_name))
	print('\n{}! Leave fields empty if you want to use the default path, name or both of them !\n'.format(colors.fg.orange))

	answers = prompt(flash_questions, style=custom_style_2)
	if answers['path']:
		default_path = answers['path'].strip('\n')
		if default_path.endswith('/'): default_path = default_path[0:len(default_path)-1]
	if answers['name']:
		flash_name = answers['name'].strip('\n')
		if flash_name.startswith('/'): flash_name = flash_name[1:]
		if not flash_name.endswith('.bin'): flash_name = '{}.bin'.format(flash_name)

	print('\n{}> {}Flashing device:\n\t- Binary file: {}{}/{} {}\n\t- USB port {}{}\n'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.lightblue, default_path, flash_name, colors.fg.lightgrey, colors.fg.lightblue, usb))

	answers = prompt(confirm_questions, style=custom_style_2)
	if answers['confirmation'] == True:
		if os.path.exists('{}'.format(default_path)):
			print('\n{}> {}Starting flash process\n{}'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.green))
			try:
				os.system('esptool.py -p {} read_flash 0x00000 0x100000 {}/{}'.format(usb, default_path, flash_name))
				print('\n{}> {}Flash process complete {}SUCCESSFULLY{} in{} {}/{}{}\n'.format(colors.fg.orange, colors.fg.lightgrey, colors.fg.green, colors.fg.lightgrey, colors.fg.cyan, default_path, flash_name, colors.fg.lightgrey))
			except Exception as e: print('{}[ERROR]{} An error occurred during the flash process. Error information: {}{}{}\n'.format(colors.fg.red, colors.fg.orange, colors.fg.lightred, e, colors.fg.lightgrey))
		else: print("\n{}Path file doesn't exist. Aborting flash process{}\n".format(colors.fg.orange, colors.fg.lightgrey))
	else: print("\n{}! Flash process canceled !{}\n".format(colors.fg.orange, colors.fg.lightgrey))

# UI questions menu
list_questions = [
	{
		'type': 'list',
		'name': 'user_option',
		'message': 'Welcome to ㄖ乂千ㄥ卂丂卄乇尺',
		'choices': ["Refresh Ports","Connect to Port", "Exit"]
	},
]
connect_questions = [
	{
		'type': 'input',
		'name': 'user_option',
		'message': 'Select index for port connection',
		"validate": IndexValidation,
		"filter": lambda val: int(val)
	},
]
action_questions = [
	{
		'type': 'list',
		'name': 'action_option',
		'message': 'What do you wanna do?',
		'choices': ["Create Backup","Erase Device", "Flash Device", "Exit"]
	},
]
backup_questions = [
	{
		'type': 'input',
		'name': 'path',
		'message': 'Set path to save the backup file?'
	},
	{
		'type': 'input',
		'name': 'name',
		'message': 'Set name to backup file?'
	},
]
erase_questions = [
	{
		'type': 'confirm',
		'name': 'erase_confirmation',
		'message': 'Are you sure you want to erase the device??'
	},
]
flash_questions = [
	{
		'type': 'input',
		'name': 'path',
		'message': 'Set path to locate binary file?'
	},
	{
		'type': 'input',
		'name': 'name',
		'message': 'Which is the binary file name?'
	},
]
confirm_questions = [
	{
		'type': 'confirm',
		'name': 'confirmation',
		'message': 'Are you sure?'
	},
]
close_questions = [
	{
		'type': 'confirm',
		'name': 'confirmation',
		'message': 'The process has finished. Do you want to go back to the menu (Y) or close script (n)?'
	},
]

def main():
	global usb_ports

	if sys.platform == "linux" or sys.platform == "linux2" or sys.platform == "darwin":
		os.system('clear')
		answers = {'user_option': 'Refresh Ports'}

		try:
			while answers['user_option'] == 'Refresh Ports':
				answers = menu_list_ports()
				if answers['user_option'] == 'Connect to Port': answers = prompt(connect_questions, style=custom_style_2)
				if answers['user_option'] == 'Exit': sys.exit()

			usb_selected = usb_ports[int(answers['user_option'])]
			answers = {'action_option': ''}
			while not answers['action_option'] == 'Exit':
				answers = menu_select_action(usb_selected)
				if answers['action_option'] == 'Create Backup': menu_backup(usb_selected)
				elif answers['action_option'] == 'Erase Device': menu_erase(usb_selected)
				elif answers['action_option'] == 'Flash Device': menu_flash(usb_selected)
				else: sys.exit()

				flag = False
				while not flag:
					answers = prompt(close_questions, style=custom_style_2)
					if answers['confirmation']:
						flag = True
						answers = {'action_option': ''}
					else: sys.exit()

		except Exception as e: print('{}> {}Script execution has been interrupted. Interruption information:{}'.format(colors.fg.blue, colors.fg.orange, e))
	else: print("\n{}! This script is not supported in Windows OS !{}\n".format(colors.fg.orange, colors.fg.lightgrey))

if __name__ == '__main__':
	main()