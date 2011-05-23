#

top = '.'
out = '__build__'

def options(ctx):
    pass

def configure(ctx):
    ctx.load('go')
    
def build(ctx):

    ctx(
        features='cgopackage',
        name ='go-ctypes',
        source='''
        pkg/ctypes/ctypes.go
        ''',
        target='bitbucket.org/binet/go-ctypes/pkg/ctypes',
        )
