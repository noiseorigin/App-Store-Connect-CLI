-- Usage:
--   osascript /absolute/path/to/get-apple-2fa-code.scpt
--   osascript /absolute/path/to/get-apple-2fa-code.scpt 90
--
-- This script polls the macOS FollowUpUI accessibility tree for a 6-digit
-- Apple two-factor code and prints the first match to stdout. If the first
-- trust dialog only shows an Allow button, the script clicks it and keeps
-- polling for the code dialog.
--
-- Requirements:
--   - The Apple 2FA dialog must be visible on this Mac.
--   - The caller (Terminal/Codex/etc.) must have Accessibility access.

on run argv
	set timeoutSeconds to 60
	if (count of argv) > 0 then
		try
			set timeoutSeconds to (item 1 of argv) as integer
		on error
			error "invalid timeout seconds: " & (item 1 of argv)
		end try
	end if

	set deadlineAt to (current date) + timeoutSeconds
	repeat while (current date) is less than deadlineAt
		set code to my findTwoFactorCode()
		if code is not "" then
			return code
		end if
		delay 1
	end repeat

	error "timed out waiting for a 2FA code in FollowUpUI. Make sure the Apple dialog is visible and Accessibility access is enabled for your terminal."
end run

on findTwoFactorCode()
	try
		tell application "System Events"
			if not (exists process "FollowUpUI") then
				return ""
			end if

			tell process "FollowUpUI"
				repeat with currentWindow in windows
					set didAdvanceTrustPrompt to my clickTrustButtonIfPresent(currentWindow)
					if didAdvanceTrustPrompt then
						delay 1
					end if

					set code to my scanElement(currentWindow)
					if code is not "" then
						my clickDoneButtonIfPresent(currentWindow)
						return code
					end if

					try
						set windowElements to entire contents of currentWindow
						repeat with currentElement in windowElements
							set code to my scanElement(currentElement)
							if code is not "" then
								my clickDoneButtonIfPresent(currentWindow)
								return code
							end if
						end repeat
					end try
				end repeat
			end tell
		end tell
	on error errMsg number errNum
		error "unable to inspect FollowUpUI via Accessibility: " & errMsg number errNum
	end try

	return ""
end findTwoFactorCode

on clickTrustButtonIfPresent(theWindow)
	if my clickButtonWithLabels(theWindow, {"Allow", "Erlauben"}) then
		return true
	end if

	return my clickRightmostButton(theWindow)
end clickTrustButtonIfPresent

on clickDoneButtonIfPresent(theWindow)
	return my clickButtonWithLabels(theWindow, {"Done", "Fertig"})
end clickDoneButtonIfPresent

on clickButtonWithLabels(theWindow, expectedLabels)
	try
		tell theWindow
			set windowButtons to buttons
		end tell
	on error
		return false
	end try

	repeat with currentButton in windowButtons
		try
			set buttonLabel to my normalizeText(name of currentButton)
			if my textMatchesAny(buttonLabel, expectedLabels) then
				if my pressButton(currentButton) then
					return true
				end if
			end if
		end try
	end repeat

	return false
end clickButtonWithLabels

on clickRightmostButton(theWindow)
	try
		tell theWindow
			set windowButtons to buttons
		end tell
	on error
		return false
	end try

	if (count of windowButtons) < 2 then
		return false
	end if

	set chosenButton to missing value
	set chosenX to -1

	repeat with currentButton in windowButtons
		try
			set buttonPosition to position of currentButton
			set buttonX to item 1 of buttonPosition
			if chosenButton is missing value or buttonX > chosenX then
				set chosenButton to currentButton
				set chosenX to buttonX
			end if
		on error
			if chosenButton is missing value then
				set chosenButton to currentButton
			end if
		end try
	end repeat

	if chosenButton is missing value then
		return false
	end if

	return my pressButton(chosenButton)
end clickRightmostButton

on pressButton(buttonReference)
	try
		tell application "System Events" to perform action "AXPress" of buttonReference
		return true
	on error
		try
			tell application "System Events" to click buttonReference
			return true
		on error
			return false
		end try
	end try
end pressButton

on textMatchesAny(candidateText, expectedLabels)
	set candidateText to candidateText as text
	repeat with expectedLabel in expectedLabels
		if candidateText is equal to (expectedLabel as text) then
			return true
		end if
	end repeat

	return false
end textMatchesAny

on scanElement(theElement)
	set candidates to {}

	try
		set end of candidates to my normalizeText(value of theElement)
	end try

	try
		set end of candidates to my normalizeText(name of theElement)
	end try

	try
		set end of candidates to my normalizeText(title of theElement)
	end try

	try
		set end of candidates to my normalizeText(description of theElement)
	end try

	repeat with candidateText in candidates
		set code to my extractFirstCode(candidateText as text)
		if code is not "" then
			return code
		end if
	end repeat

	return ""
end scanElement

on normalizeText(candidateValue)
	if candidateValue is missing value then
		return ""
	end if

	if class of candidateValue is list then
		set previousDelimiters to AppleScript's text item delimiters
		set AppleScript's text item delimiters to " "
		set joinedText to candidateValue as text
		set AppleScript's text item delimiters to previousDelimiters
		return joinedText
	end if

	return candidateValue as text
end normalizeText

on extractFirstCode(sourceText)
	set sourceText to sourceText as text
	try
		return do shell script "/bin/echo " & quoted form of sourceText & " | /usr/bin/grep -Eo '[0-9]{6}' | /usr/bin/head -n1"
	on error
		return ""
	end try
end extractFirstCode
