# Translation guide

Thank you for helping to translate this bot into many different languages. If you chose to translate this bot, please try to test every possible case that you can think of. As time passes, new features will be added and your translation could become out of date. It would be great, if you could update your language's translation if you notice any weird changes. 

## Quick and dirty summary
* Duplicate `en.toml` to your localization and edit string by string.
* Do not translate commands in the text! I.e. `/balance` stays `/balance`.
* Pay attention to every single `"""` and `%s` or `%d`. 
* Your end resiult should have exactly the same number of lines as `en.toml`.
* Start sentences with `C`apital letters, end them with a full stop`.`

## General 
* The bot checks the language settings of each Telegram user and translate the interaction with the user (private chats, **inline commands?**) to the user's language, if a translation is available. Otherwise, it will default to english. All messages in groups will be english. If the user does not have a language setting, it will default to english.
* For now, all `/commands` are in english. That means that all `/command` references in the help messages should remain english for now. We plan to implement localized commands, which is why you will find the strings in the translation files. Please chose simple, single-worded, lower-case, for the command translations.
* Please use a spell checker, like Google Docs to check your final translation. Thanks :)

## Language
* Please use a "kind" and "playful" tone in your translations. We do not have to be very dry and technical in tone. Please use a respectful language.
* Please remember who the prototypical user is: a non-technical fruit-selling lady in Brazil that wants to sell Mangos for Satoshis. 

## Standards
* Please "fork" your translation from the english translation file `en.toml`. Simply copy the file, rename it to your language code (look it up on Google if you're unsure) and start editing :)
* Please use only "sat" as a denominator for amounts, do not use the plural form "sats". 
* Please choose an appropriate expression for "Amount" and keep it across the entire translation.
* Please reuse all Emojis in the same location and order as the original text.
* Do not add line breaks. All translations should have the same number of lines.
* Please use english words for Bitcoin- and Lightning-native concepts like "Lightning", "Wallet", "Invoice", "Tip", and other technical terms like "Log", "Bot", etc. **IF** your language does not have a widely-used and recognized alternative for it. If most software in your language uses another word instead of "Wallet" for example, then we should also use that. 
* For fixed english terms like "Tip" I recommend using the english version and giving a translation in parenthesis like "... Tips (*kleine Beträge*) senden kann". The text in *italic* is the next best translation of "Tips"


## Technical
* Every string should be wrapped in three quotes `"""`
* Strings can span over multiple lines.
* Every string variable found in the original english language file should be translated. If a specific string is missing in a translation, the english version will be used for that particular string.
* Every language has their own translation file. The file for english is `en.toml`. 

* Headings to many sections are **bold** starting and ending with asterix `*`. Italic starts and ends with an underscore `_`.
* Command examples are in `code format` starting end ending with ``` ` ```

## Pleaceholders
* Symbols like `%s`, `%d`, `%f` are meant as placeholders for other bits of text, numbers, floats. Please reuse them in every string you translate.
* Please do not change the order of the placeholders in your translation. It would break things.
* Please do not use modifiers for **bold**, *italic*, and others around the placeholders. We are using MarkdownV1 and it would break things. Do not do `_%s_` for example.

## GitHub infos
* Please submit translations as a GitHub pull-request. This way, you can easily work with others and review each other. To submit a pull-request, you need a Github account. Then, fork the entire project (using the button in the upper-right corner). 
* Then, create a new branch for your translation. Do this using the Github UI or via the terminal inside the project repository: `git checkout -b translation_es` for example. Then, create the appropriate language file and put it in the translations folder. Then, add it to the branch with by navigating to the translations folder `git add es.toml` and `git commit -m 'add spanish'`. Finally, push the branch to your fork `git push --set-upstream origin translation_es`. When done, open a pull-request in the *original github repo* and select your forked branch. 
* Good luck :)