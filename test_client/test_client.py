#!/bin/python3
import random, string, json, uuid, requests, hashlib, time

BASE_URL = 'http://localhost:8090'
AUTH_FULL_URL = BASE_URL + '/auth/full'
AUTH_GET_WALLET_STATE_URL = BASE_URL + '/auth/get-wallet-state'
REGISTER_URL = BASE_URL + '/signup'
WALLET_STATE_URL = BASE_URL + '/wallet-state'

def wallet_state_sequence(wallet_state):
  if 'deviceId' not in wallet_state:
    return 0
  return wallet_state['lastSynced'][wallet_state['deviceId']]

def download_key(password):
    return hashlib.sha256(password.encode('utf-8')).hexdigest()

class Client():
  def _validate_new_wallet_state(self, new_wallet_state):
    if self.wallet_state is None:
      # All of the validations here are in reference to what the device already
      # has. If this device is getting a wallet state for the first time, there
      # is no basis for comparison.
      return True

    # Make sure that the new sequence is overall later.
    if wallet_state_sequence(new_wallet_state) <= wallet_state_sequence(self.wallet_state):
      return False

    for dev_id in self.wallet_state['lastSynced']:
      if dev_id == self.device_id:
        # Check if the new wallet has the latest changes from this device
        if new_wallet_state['lastSynced'][dev_id] != self.wallet_state['lastSynced'][dev_id]:
          return False
      else:
        # Check if the new wallet somehow regressed on any of the other devices
        # This most likely means a bug in another client
        if new_wallet_state['lastSynced'][dev_id] < self.wallet_state['lastSynced'][dev_id]:
          return False

    return True

  def __init__(self):
    # Represents normal client behavior (though a real client will of course save device id)
    self.device_id = str(uuid.uuid4())

    self.auth_token = 'bad token'

    self.wallet_state = None

  def new_wallet(self, email, password):
    # Obviously not real behavior
    self.public_key = ''.join(random.choice(string.hexdigits) for x in range(32))

    # camel-cased to ease json interop
    self.wallet_state = {'lastSynced': {}, 'encryptedWallet': ''}

    # TODO - actual encryption with password
    self._encrypted_wallet_local_changes = ''

    self.email = email
    self.password = password

  def register(self):
    body = json.dumps({
      'token': self.auth_token,
      'publicKey': self.public_key,
      'deviceId': self.device_id,
      'email': self.email,
    })
    response = requests.post(REGISTER_URL, body)
    if response.status_code != 201:
      print ('Error', response.status_code)
      print (response.content)
      return
    print ("Registered")

  def get_download_auth_token(self, email, password):
    body = json.dumps({
      'email': email,
      'downloadKey': download_key(password),
      'deviceId': self.device_id,
    })
    response = requests.post(AUTH_GET_WALLET_STATE_URL, body)
    if response.status_code != 200:
      print ('Error', response.status_code)
      print (response.content)
      return
    self.auth_token = json.loads(response.content)['token']
    self.public_key = json.loads(response.content)['publicKey']
    print ("Got auth token: ", self.auth_token)
    print ("Got public key: ", self.public_key)

    self.email = email
    self.password = password

  def get_full_auth_token(self):
    if not self.wallet_state:
      print ("No wallet state, thus no access to private key (or so we pretend for this demo), thus we cannot create a signature")
      return

    body = json.dumps({
      'tokenRequestJSON': json.dumps({'deviceId': self.device_id, 'requestTime': int(time.time())}),
      'publicKey': self.public_key,
      'signature': 'Good Signature',
    })
    response = requests.post(AUTH_FULL_URL, body)
    if response.status_code != 200:
      print ('Error', response.status_code)
      print (response.content)
      return
    self.auth_token = json.loads(response.content)['token']
    print ("Got auth token: ", self.auth_token)

  def get_wallet_state(self):
    # A new wallet state, last updated by this device, with the updated sequence.
    # Don't set it to our own wallet_state just yet because we don't know if it will accept the new sequence number.
    params = {
      'token': self.auth_token,
      'publicKey': self.public_key,
      'deviceId': self.device_id,
    }
    response = requests.get(WALLET_STATE_URL, params=params)
    if response.status_code != 200:
      print ('Error', response.status_code)
      print (response.content)
      return

    if json.loads(response.content)['signature'] != "Good Signature":
      print ('Error - bad signature on new wallet')
      print (response.content)
      return
    if response.status_code != 200:
      print ('Error', response.status_code)
      print (response.content)
      return

    # In reality, we'd examine, merge, verify, validate etc this new wallet state.
    new_wallet_state = json.loads(json.loads(response.content)['bodyJSON'])
    if self.wallet_state != new_wallet_state and not self._validate_new_wallet_state(new_wallet_state):
      print ('Error - new wallet does not validate')
      print (response.content)
      return

    if self.wallet_state is None:
      # This is if we're getting a wallet_state for the first time. Initialize
      # the local changes.
      self._encrypted_wallet_local_changes = ''
    self.wallet_state = new_wallet_state

    print ("Got latest walletState: ", self.wallet_state)

  def post_wallet_state(self):
    # A new wallet state, last updated by this device, with the updated sequence.
    # Don't set it to our own wallet_state just yet because we don't know if it will accept the new sequence number.
    if self.wallet_state:
      submitted_wallet_state = {
        "deviceId": self.device_id,
        "lastSynced": dict(self.wallet_state['lastSynced']),
        "encryptedWallet": self.cur_encrypted_wallet(),
      }
      submitted_wallet_state['lastSynced'][self.device_id] = wallet_state_sequence(self.wallet_state) + 1
    else:
      # If we have no self.wallet_state, we shouldn't be able to have a full
      # auth token, so this code path is just to demonstrate an auth failure
      submitted_wallet_state = {
        "deviceId": self.device_id,
        "lastSynced": {self.device_id: 1},
        "encryptedWallet": self.cur_encrypted_wallet(),
      }

    body = json.dumps({
      'token': self.auth_token,
      'bodyJSON': json.dumps(submitted_wallet_state),
      'publicKey': self.public_key,
      'downloadKey': download_key(self.password),
      'signature': 'Good Signature',
    })
    response = requests.post(WALLET_STATE_URL, body)

    if response.status_code == 200:
      # Our local changes are no longer local, so we reset them
      self._encrypted_wallet_local_changes = ''
      print ('Successfully updated wallet state')
    elif response.status_code == 409:
      print ('Wallet state out of date. Getting updated wallet state. Try again.')
    else:
      print ('Error', response.status_code)
      print (response.content)
      return

    if json.loads(response.content)['signature'] != "Good Signature":
      print ('Error - bad signature on new wallet')
      print (response.content)
      return

    # In reality, we'd examine, merge, verify, validate etc this new wallet state.
    new_wallet_state = json.loads(json.loads(response.content)['bodyJSON'])
    if submitted_wallet_state != new_wallet_state and not self._validate_new_wallet_state(new_wallet_state):
      print ('Error - new wallet does not validate')
      print (response.content)
      return

    self.wallet_state = new_wallet_state

    print ("Got new walletState: ", self.wallet_state)

  def change_encrypted_wallet(self):
    if not self.wallet_state:
      print ("No wallet state, so we can't add to it yet.")
      return

    self._encrypted_wallet_local_changes += ':' + ''.join(random.choice(string.hexdigits) for x in range(4))

  def cur_encrypted_wallet(self):
    if not self.wallet_state:
      print ("No wallet state, so no encrypted wallet.")
      return

    # The local changes on top of whatever came from the server
    # If we pull new changes from server, we "rebase" these on top of it
    # If we push changes, the full "rebased" version gets committed to the server
    return self.wallet_state['encryptedWallet'] + self._encrypted_wallet_local_changes
