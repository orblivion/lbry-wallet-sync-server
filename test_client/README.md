# Test Client

A couple example flows so it's clear how it works.

## Initial setup and account recovery

```
>>> import test_client
>>> c1 = test_client.Client()
```

Create a new wallet locally and authenticate based on the newly created public key (the email and password are not used just yet)

```
>>> c1.new_wallet('email@example.com', '123')
>>> c1.get_full_auth_token()
Got auth token:  787cefea147f3a7b38e1b9fda49490371b52a3b7077507364854b72c3538f94e
```

Post the wallet along with the downloadKey. The downloadKey is based on the password. It's the same password that will be used (in the full implementation) to encrypt the wallet. This is why we are sending it with the wallet state. We want to keep everything related to the user's password consistent.

```
>>> c1.post_wallet_state()
Successfully updated wallet state
Got new walletState:  {'deviceId': 'e0349bc4-7e7a-48a2-a562-6c530b28a350', 'lastSynced': {'e0349bc4-7e7a-48a2-a562-6c530b28a350': 1}, 'encryptedWallet': ''}
```

Note that every time a client posts, the server sends back the latest wallet state, whether or not the posted wallet state was rejected for being out of sequence. More on this below.

Send the email address

```
>>> c1.register()
Registered
```

Now let's set up a second device

```
>>> c2 = test_client.Client()
```

Gets limited-scope auth token (which includes pubkey) based on email address and downloadKey (which comes from password). This token only allows downloading a wallet state (thus the "downloadKey").

```
>>> c2.get_download_auth_token('email@example.com', '123')
Got auth token:  fd3f4074e6f1b2401b33e21ce5f69d93255680b37c334b6a4e8ea6385b454b0b
Got public key:  eeA0FfE5E57E3647524759CA9D7c7Cb1
>>>
```

Full auth token requires signature, which requires the wallet, which we don't have yet. (For demo we have a fake signature check, so this restriction is faked by the client)

```
>>> c2.get_full_auth_token()
No wallet state, thus no access to private key (or so we pretend for this demo), thus we cannot create a signature
```

Get the wallet state.

```
>>> c2.get_wallet_state()
Got latest walletState:  {'deviceId': 'e0349bc4-7e7a-48a2-a562-6c530b28a350', 'lastSynced': {'e0349bc4-7e7a-48a2-a562-6c530b28a350': 1}, 'encryptedWallet': ''}
```

The download-only auth token doesn't allow posting a wallet.

```
>>> c2.post_wallet_state()
Error 403
b'{"error":"Forbidden: Scope"}\n'
```

But, we can get the full auth token now that we downloaded the wallet. In the full implementation, the wallet would be encrypted with the password. This means that somebody who merely intercepts the public key and download key wouldn't be able to do this step.

```
>>> c2.get_full_auth_token()
Got auth token:  4b19739a66f55aff5b7e0f1375c42f41d944b5175f5c5d32b35698a360bb0e5b
>>> c2.post_wallet_state()
Successfully updated wallet state
Got new walletState:  {'deviceId': '2ede3f32-4e65-4312-8b89-3b6bde0c5d8e', 'lastSynced': {'e0349bc4-7e7a-48a2-a562-6c530b28a350': 1, '2ede3f32-4e65-4312-8b89-3b6bde0c5d8e': 2}, 'encryptedWallet': ''}
```

# Handling conflicts

Changes here are represented by 4 random characters separated by colons. The sequence of the changes is relevant to the final state of the wallet. Our goal is to make sure that all clients have all of the changes in the same order. This will thus demonstrate how clients can implement a "rebase" behavior when there is a conflict. In a full implementation, there would also be a system to resolve merge conflicts, but that is out of scope here.

First, create a local change and post it

```
>>> c1.change_encrypted_wallet()
>>> c1.cur_encrypted_wallet()
':f801'
>>> c1.post_wallet_state()
Successfully updated wallet state
Got new walletState:  {'deviceId': 'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938', 'lastSynced': {'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938': 2}, 'encryptedWallet': ':f801'}
>>> c1.cur_encrypted_wallet()
':f801'
```

The other client gets the update and sees the same thing locally:

```
>>> c2.get_wallet_state()
Got latest walletState:  {'deviceId': 'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938', 'lastSynced': {'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938': 2}, 'encryptedWallet': ':f801'}
>>> c2.cur_encrypted_wallet()
':f801'
```

Now, both clients make different local changes and both try to post them

```
>>> c1.change_encrypted_wallet()
>>> c2.change_encrypted_wallet()
>>> c1.cur_encrypted_wallet()
':f801:576b'
>>> c2.cur_encrypted_wallet()
':f801:dDE7'
>>> c1.post_wallet_state()
Successfully updated wallet state
Got new walletState:  {'deviceId': 'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938', 'lastSynced': {'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938': 3}, 'encryptedWallet': ':f801:576b'}

>>> c2.post_wallet_state()
Wallet state out of date. Getting updated wallet state. Try again.
Got new walletState:  {'deviceId': 'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938', 'lastSynced': {'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938': 3}, 'encryptedWallet': ':f801:576b'}
```

Client 2 gets a conflict, and the server sends it the updated wallet state that was just created by Client 1 (to save an extra request to `getWalletState`).

Its local change still exists, but now it's on top of client 1's latest change. (In a full implementation, this is where conflict resolution might take place.)

```
>>> c2.cur_encrypted_wallet()
':f801:576b:dDE7'
```

Client 2 tries again to post, and it succeeds. Client 1 receives it.

```
>>> c2.post_wallet_state()
Successfully updated wallet state
Got new walletState:  {'deviceId': '127e0045-425c-4dd8-a742-90cd52b9377b', 'lastSynced': {'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938': 3, '127e0045-425c-4dd8-a742-90cd52b9377b': 4}, 'encryptedWallet': ':f801:576b:dDE7'}
>>> c1.get_wallet_state()
Got latest walletState:  {'deviceId': '127e0045-425c-4dd8-a742-90cd52b9377b', 'lastSynced': {'f9acb3bb-ec3b-43f9-9c93-b279b9fdc938': 3, '127e0045-425c-4dd8-a742-90cd52b9377b': 4}, 'encryptedWallet': ':f801:576b:dDE7'}
>>> c1.cur_encrypted_wallet()
':f801:576b:dDE7'
```
